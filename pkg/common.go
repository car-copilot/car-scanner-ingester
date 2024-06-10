package obd2influx

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog/log"
)

var db *sql.DB
var influxClient influxdb2.Client
var Config = ConfigStruct{}

var K = koanf.New(".")

type ConfigStruct struct {
	ListPids *bool `koanf:"listPids"`
	DryRun   *bool `koanf:"dryRun"`
	Owners   map[string]Vehicles
	Influx   struct {
		Token string `koanf:"token"`
		Url   string `koanf:"url"`
		Org   string `koanf:"org"`
	} `koanf:"influxdb"`
	Vehicles Vehicles `koanf:"vehicles"`
	Http     struct {
		Host string `koanf:"host"`
		Port int64  `koanf:"port"`
	} `koanf:"http"`
}

type Convetion struct {
	From string `koanf:"from"`
	To   string `koanf:"to"`
}

type Vehicle struct {
	Brand      string               `koanf:"brand"`
	Model      string               `koanf:"model"`
	Engine     string               `koanf:"engine"`
	Year       int                  `koanf:"year"`
	Path       string               `koanf:"path"`
	Interval   time.Duration        `koanf:"interval"`
	IgnorePids []string             `koanf:"ignorePids"`
	PidMap     map[string]string    `koanf:"pidMap"`
	Convert    map[string]Convetion `koanf:"convert"`
}

type Vehicles []Vehicle

var VehicleRegex = regexp.MustCompile(`(\w+) - ([a-zA-Z0-9. ]+)(- ([a-zA-Z0-9. ]+))?(\((\d+)\))?`)

func InitWithConfigFile(path string) {
	f := file.Provider(path)
	if err := K.Load(f, yaml.Parser()); err != nil {
		log.Fatal().Err(err)
	}

	K.Load(env.Provider("CSI_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "CSI_")), "_", ".", -1)
	}), nil)

	if err := K.Unmarshal("influxdb", &Config.Influx); err != nil {
		log.Fatal().Err(err)
	}

	ConnectPg()

	influxClient = influxdb2.NewClient(Config.Influx.Url, Config.Influx.Token)
	defer influxClient.Close()

	Config.Owners = make(map[string]Vehicles, 0)
	Config.Vehicles = make(Vehicles, 0)

	if err := K.Unmarshal("http", &Config.Http); err != nil {
		log.Fatal().Err(err)
	}

	if err := K.Unmarshal("vehicles", &Config.Vehicles); err != nil {
		log.Fatal().Err(err)
	}

	MergeVehicleConfigs()
}

func Init() {
	InitWithConfigFile("config.yaml")
}

func FindBestVehicleMatch(search string) (Vehicle, error) {

	matches := VehicleRegex.FindStringSubmatch(search)
	if matches == nil {
		log.Warn().Msgf("No match found for %s", search)
		return Vehicle{}, fmt.Errorf("no match found for %s", search)
	}
	// Parse year
	year, err := strconv.Atoi(matches[6])
	if err != nil {
		log.Warn().Err(err)
	}

	searchVehicle := Vehicle{
		Brand:  strings.TrimSpace(matches[1]),
		Model:  strings.TrimSpace(matches[2]),
		Engine: strings.TrimSpace(matches[4]),
		Year:   year,
	}

	for _, vehicle := range Config.Vehicles {
		if searchVehicle.Brand == vehicle.Brand && searchVehicle.Model == vehicle.Model && searchVehicle.Engine == vehicle.Engine && searchVehicle.Year == vehicle.Year {
			vehicle.Path = searchVehicle.Path
			return vehicle, nil
		}
	}
	return searchVehicle, fmt.Errorf("no match found for %s", search)
}

func Contains(slice []string, element string) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}

func MergeSlicesUnique(s1 []string, s2 []string) []string {
	merged := make([]string, 0)
	merged = append(merged, s1...)
	for _, element := range s2 {
		if !contains(merged, element) {
			merged = append(merged, element)
		}
	}
	return merged
}

func MergeMaps(m1 map[string]string, m2 map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for key, value := range m2 {
		merged[key] = value
	}
	return merged
}

func MergeVehicleConfigs() {
	for i, vehicle := range Config.Vehicles {
		mergedPids := MergeMaps(K.MustStringMap("pidMap"), vehicle.PidMap)
		mergedIgnorePids := MergeSlicesUnique(K.MustStrings("ignorePids"), vehicle.IgnorePids)

		vehicle.PidMap = mergedPids
		vehicle.IgnorePids = mergedIgnorePids
		Config.Vehicles[i] = vehicle
	}
	log.Info().Msgf("Merged: %v", Config.Vehicles)
}
