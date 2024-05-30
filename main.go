package main

import (
	"context"
	"database/sql"
	"flag"
	"math"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	obd2influx "github.com/Obito1903/obd2influx/pkg"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var k = koanf.New(".")

var db *sql.DB

var folderRegex = regexp.MustCompile(`(\w+) - ([a-zA-Z0-9. ]+)(- ([a-zA-Z0-9. ]+))?(\((\d+)\))?`)

func connectPg() {
	var err error
	db, err = sql.Open("postgres", k.MustString("pg.connection"))
	if err != nil {
		log.Fatal().Err(err)
	}

}

func findBestVehicleMatch(vehicles obd2influx.Vehicles, search obd2influx.Vehicle) obd2influx.Vehicle {

	for _, vehicle := range vehicles {
		if search.Brand == vehicle.Brand && search.Model == vehicle.Model && search.Engine == vehicle.Engine && search.Year == vehicle.Year {
			vehicle.Path = search.Path
			return vehicle
		}
	}
	return search
}

func contains(slice []string, element string) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}

func mergeSlicesUnique(s1 []string, s2 []string) []string {
	merged := make([]string, 0)
	merged = append(merged, s1...)
	for _, element := range s2 {
		if !contains(merged, element) {
			merged = append(merged, element)
		}
	}
	return merged
}

func mergeMaps(m1 map[string]string, m2 map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for key, value := range m2 {
		merged[key] = value
	}
	return merged
}

func registerTrip(vehicle obd2influx.Vehicle, owner string, start time.Time, end time.Time) string {
	// Check if user exists
	var userId int
	var bucket string
	err := db.QueryRow("SELECT owner_id, bucket FROM owners WHERE name = $1", owner).Scan(&userId, &bucket)
	if err != nil {
		log.Err(err).Send()
		bucket = strings.ToLower(owner)
		log.Info().Msgf("Owner %s notfound", owner)
		log.Info().Msgf("Creating owner %s with bucket %s", owner, bucket)
		// Create user
		_, err := db.Exec("INSERT INTO owners (name, bucket) VALUES ($1, $2)", owner, bucket)
		if err != nil {
			log.Fatal().Err(err)
		}
		err = db.QueryRow("SELECT id FROM owners WHERE name = $1", owner).Scan(&userId, &bucket)
		if err != nil {
			log.Fatal().Err(err)
		}
	}

	// Check if vehicle exists
	var vehicleId int
	err = db.QueryRow("SELECT car_id FROM car WHERE brand = $1 AND model = $2 AND year = $3 AND engine = $4", vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.Engine).Scan(&vehicleId)
	if err != nil {
		log.Err(err).Send()
		log.Info().Msgf("Vehicle %s %s %d %s not found", vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.Engine)
		log.Info().Msgf("Creating vehicle %s %s %d %s", vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.Engine)
		// Create vehicle
		_, err := db.Exec("INSERT INTO car (brand, model, year, engine) VALUES ($1, $2, $3, $4)", vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.Engine)
		if err != nil {
			log.Fatal().Err(err)
		}
		err = db.QueryRow("SELECT id FROM car WHERE brand = $1 AND model = $2 AND year = $3 AND engine = $4", vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.Engine).Scan(&vehicleId)
		if err != nil {
			log.Fatal().Err(err)
		}
	}

	// Check if trip exists
	var tripId int
	err = db.QueryRow("SELECT trip_id FROM trip WHERE begin_timestamp = $1 AND end_timestamp = $2 AND car_id = $3 AND owner_id = $4", start, end, vehicleId, userId).Scan(&tripId)
	if err == nil {
		log.Info().Msgf("Trip already registered with id %d", tripId)
		return bucket
	}

	// Register trip
	_, err = db.Exec("INSERT INTO trip (begin_timestamp, end_timestamp, car_id, owner_id) VALUES ($1, $2, $3, $4)", start, end, vehicleId, userId)
	if err != nil {
		log.Fatal().Err(err)
	}
	return bucket
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	f := file.Provider("config.yaml")
	if err := k.Load(f, yaml.Parser()); err != nil {
		log.Fatal().Err(err)
	}

	listPids := flag.Bool("list-pids", false, "List all PIDs")
	dryRun := flag.Bool("dry-run", false, "Dry run")
	flag.Parse()

	token := k.String("influxdb.token")
	url := k.String("influxdb.url")
	org := k.String("influxdb.org")

	connectPg()

	client := influxdb2.NewClient(url, token)
	defer client.Close()

	var owners = make(map[string]obd2influx.Vehicles, 0)
	var vehiclesConf = make(obd2influx.Vehicles, 0)

	if err := k.Unmarshal("vehicles", &vehiclesConf); err != nil {
		log.Fatal().Err(err)
	}

	// Read List folders in path
	folders, err := os.ReadDir(k.String("path"))
	if err != nil {
		log.Fatal().Err(err)
	}
	for _, folder := range folders {
		if !folder.IsDir() {
			log.Warn().Msgf("Skipping %s, not a directory", folder.Name())
			continue
		}
		owner := folder.Name()
		// Check if owner exists
		if _, ok := owners[owner]; !ok {
			owners[owner] = make(obd2influx.Vehicles, 0)
		}

		files, err := os.ReadDir(path.Join(k.String("path"), folder.Name()))
		if err != nil {
			log.Fatal().Err(err)
		}

		for _, file := range files {
			if !file.IsDir() {
				log.Warn().Msgf("Skipping %s, not a directory", file.Name())
				continue
			}
			// Parse vehicle from folder name
			matches := folderRegex.FindStringSubmatch(file.Name())
			if matches == nil {
				log.Warn().Msgf("Skipping %s, not a valid folder name", file.Name())
				continue
			}
			// Parse year
			year, err := strconv.Atoi(matches[6])
			if err != nil {
				log.Warn().Err(err)
			}

			vehicle := obd2influx.Vehicle{
				Brand:  strings.TrimSpace(matches[1]),
				Model:  strings.TrimSpace(matches[2]),
				Engine: strings.TrimSpace(matches[4]),
				Year:   year,
				Path:   path.Join(k.String("path"), folder.Name(), file.Name()),
			}

			log.Debug().Msgf("Found vehicle %s %s %d %s for %s in %s", vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.Engine, owner, vehicle.Path)

			vehicle = findBestVehicleMatch(vehiclesConf, vehicle)

			owners[owner] = append(owners[owner], vehicle)
		}
	}

	for owner, vehicles := range owners {
		for _, vehicle := range vehicles {
			interval := vehicle.Interval
			mergedPids := mergeMaps(k.MustStringMap("pidMap"), vehicle.PidMap)
			mergedIgnorePids := mergeSlicesUnique(k.MustStrings("ignorePids"), vehicle.IgnorePids)

			log.Debug().Msgf("vehicle Path: %s", vehicle.Path)
			files, err := os.ReadDir(vehicle.Path)
			if err != nil {
				log.Fatal().Err(err)
			}

			log.Info().Msgf("Processing vehicle %s %s for %s", vehicle.Brand, vehicle.Model, owner)
			for _, file := range files {
				log.Debug().Msgf("Processing file %s", file.Name())

				date, err := time.ParseInLocation("2006-01-02 15-04-05.csv", file.Name(), time.Local)
				if err != nil {
					log.Fatal().Err(err)
				}
				raw_points, endDate := obd2influx.ReadCsv(path.Join(vehicle.Path, file.Name()), date, mergedPids, mergedIgnorePids, vehicle.Convert)

				log.Info().Msgf("Registering trip in database")
				bucket := registerTrip(vehicle, owner, date, endDate)

				points := obd2influx.GroupDataPoint(raw_points, interval)
				influxPoints := []*write.Point{}
				for _, point := range points {
					influxPoints = append(influxPoints, point.ToInfluxPoint())
				}
				if *listPids {
					mapPids := make(map[string]bool)
					for _, point := range points {
						if _, ok := mapPids[point.Pid]; !ok {
							mapPids[point.Pid] = true
							log.Info().Msgf("%s | %s", point.Pid, point.Unit)
						}
					}
				} else {
					if !*dryRun {
						deleteAPI := client.DeleteAPI()
						orgObj, err := client.OrganizationsAPI().FindOrganizationByName(context.Background(), org)
						if err != nil {
							log.Fatal().Err(err)
						}
						bucketObj, err := client.BucketsAPI().FindBucketByName(context.Background(), bucket)
						if err != nil {
							log.Fatal().Err(err)
						}
						if bucketObj == nil {
							log.Info().Msgf("Creating bucket %s in org %s", bucket, orgObj.Name)

							bucketObj, err = client.BucketsAPI().CreateBucketWithName(context.Background(), orgObj, bucket)
							if err != nil {
								log.Fatal().Err(err)
							}
						}

						log.Info().Msgf("Deleting data from %s to %s for bucket %s in org %s", date, endDate, bucketObj.Name, orgObj.Name)
						deleteAPI.Delete(context.Background(), orgObj, bucketObj, date, endDate, "_measurement=\"*\"")
						writeAPI := client.WriteAPIBlocking(org, bucket)
						log.Info().Msgf("Writing %d points to InfluxDB", len(points))
						log.Info().Msgf("Data available at https://car-grafana.obito.fr/d/edmyb3fp6zocge/car-scanner?orgId=1&var-car=%s&from=%d&to=%d&refresh=10s", bucket, date.UnixMilli(), endDate.UnixMilli())
						for i, point := range influxPoints {
							progess := (float64(i) / float64(len(influxPoints))) * 100
							_, dec := math.Modf(progess)
							if dec == 0 {
								log.Info().Msgf("Progress: %.2f%%", progess)
							}
							if err := writeAPI.WritePoint(context.Background(), point); err != nil {
								log.Fatal().Err(err)
							}
						}
					}
				}
			}
		}
	}
}
