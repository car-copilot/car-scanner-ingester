package main

import (
	"context"
	"os"

	obd2influx "github.com/Obito1903/obd2influx/pkg"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	points := obd2influx.ReadCsv("test/2023-12-12 17-35-36.csv")

	token := os.Getenv("INFLUXDB_TOKEN")
	url := "http://localhost:8086"
	client := influxdb2.NewClient(url, token)
	defer client.Close()

	org := "obicorp"
	bucket := "local"
	writeAPI := client.WriteAPIBlocking(org, bucket)
	for _, point := range points {
		if err := writeAPI.WritePoint(context.Background(), point); err != nil {
			log.Fatal().Err(err)
		}
	}
}
