package obd2influx

import (
	"context"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"time"

	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/rs/zerolog/log"
)

func ProcessCsv(reader io.Reader, date time.Time, vehicle Vehicle, mergedPids map[string]string, mergedIgnorePids []string, owner string) {
	raw_points, endDate := ReadCsv(reader, date, mergedPids, mergedIgnorePids, vehicle.Convert)

	log.Info().Msgf("Registering trip in database")
	bucket := registerTrip(vehicle, owner, date, endDate)

	interval := findMeanTimeInterval(raw_points)

	log.Info().Msgf("interval: %s", interval)

	points := GroupDataPoint(raw_points, interval)
	influxPoints := []*write.Point{}
	for _, point := range points {
		influxPoints = append(influxPoints, point.ToInfluxPoint())
	}
	if *Config.ListPids {
		mapPids := make(map[string]bool)
		for _, point := range points {
			if _, ok := mapPids[point.Pid]; !ok {
				mapPids[point.Pid] = true
				log.Info().Msgf("%s | %s", point.Pid, point.Unit)
			}
		}
	} else {
		if !*Config.DryRun {
			deleteAPI := influxClient.DeleteAPI()
			orgObj, err := influxClient.OrganizationsAPI().FindOrganizationByName(context.Background(), Config.Influx.Org)
			if err != nil {
				log.Fatal().Err(err)
			}
			bucketObj, err := influxClient.BucketsAPI().FindBucketByName(context.Background(), bucket)
			if err != nil {
				log.Fatal().Err(err)
			}
			if bucketObj == nil {
				log.Info().Msgf("Creating bucket %s in org %s", bucket, orgObj.Name)

				bucketObj, err = influxClient.BucketsAPI().CreateBucketWithName(context.Background(), orgObj, bucket)
				if err != nil {
					log.Fatal().Err(err)
				}
			}

			log.Info().Msgf("Deleting data from %s to %s for bucket %s in org %s", date, endDate, bucketObj.Name, orgObj.Name)
			deleteAPI.Delete(context.Background(), orgObj, bucketObj, date, endDate, "")
			writeAPI := influxClient.WriteAPIBlocking(Config.Influx.Org, bucket)
			log.Info().Msgf("Writing %d points to InfluxDB", len(points))
			log.Info().Msgf("Data available at https://car-grafana.obito.fr/d/edmyb3fp6zocge/car-scanner?orgId=1&var-car=%s&from=%d&to=%d&refresh=10s", bucket, date.UnixMilli(), endDate.UnixMilli())
			for i, point := range influxPoints {
				progress := math.Round(float64(i) / float64(len(influxPoints)) * 100)
				modulo := (i % (len(influxPoints) / 100))
				if modulo == 0 {
					log.Info().Msgf("Progress: %.2f%%", progress)
				}
				if err := writeAPI.WritePoint(context.Background(), point); err != nil {
					log.Fatal().Err(err)
				}
			}
		}
	}
}

func ProcessFile(file fs.DirEntry, vehicle Vehicle, mergedPids map[string]string, mergedIgnorePids []string, owner string, interval time.Duration) {
	date, err := time.ParseInLocation("2006-01-02 15-04-05.csv", file.Name(), time.Local)
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Info().Msgf("Processing file %s", file.Name())
	reader, err := os.Open(path.Join(vehicle.Path, file.Name()))
	if err != nil {
		log.Fatal().Err(err)
	}
	ProcessCsv(reader, date, vehicle, mergedPids, mergedIgnorePids, owner)
}
