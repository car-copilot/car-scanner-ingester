package obd2influx

import (
	"database/sql"
	"regexp"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/knadh/koanf/parsers/yaml"
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

	if err := K.Unmarshal("influxdb", &Config.Influx); err != nil {
		log.Fatal().Err(err)
	}

	ConnectPg()

	influxClient = influxdb2.NewClient(Config.Influx.Url, Config.Influx.Token)
	defer influxClient.Close()

	Config.Owners = make(map[string]Vehicles, 0)
	Config.Vehicles = make(Vehicles, 0)

	if err := K.Unmarshal("vehicles", &Config.Vehicles); err != nil {
		log.Fatal().Err(err)
	}
}

func Init() {
	InitWithConfigFile("config.yaml")
}

func FindBestVehicleMatch(vehicles Vehicles, search Vehicle) Vehicle {

	for _, vehicle := range vehicles {
		if search.Brand == vehicle.Brand && search.Model == vehicle.Model && search.Engine == vehicle.Engine && search.Year == vehicle.Year {
			vehicle.Path = search.Path
			return vehicle
		}
	}
	return search
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
