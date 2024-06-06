package obd2influx

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func ingest(w http.ResponseWriter, r *http.Request) {
	log.Info().Msgf("Received request to ingest data.")

	// Parse the request body.
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to parse request body.")
		return
	}

	// Extract the file from the request.
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Error().Err(err).Msgf("Failed to extract file from request.")
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		log.Error().Msgf("No filename provided.")
		return
	}

	car := r.FormValue("car")
	if car == "" {
		log.Error().Msgf("No car provided.")
		return
	}

	ownerMail := r.FormValue("owner")
	if ownerMail == "" {
		log.Error().Msgf("No owner provided.")
		return
	}

	date, err := time.ParseInLocation("2006-01-02 15-04-05.csv", filename, time.Local)
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Debug().Msgf("Processing car %s for %s at %v", car, ownerMail, date)
	log.Debug().Msgf("file name %s", filename)

	currentVehicle, err := FindBestVehicleMatch(car)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to find best vehicle match.")
		return
	}

	owner, err := getOwnerFromMail(ownerMail)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get owner from mail.")
		return
	}

	go ProcessCsv(file, date, currentVehicle, currentVehicle.PidMap, currentVehicle.IgnorePids, owner)

	defer file.Close()
}

func Serve() {
	log.Info().Msgf("Starting our simple http server.")

	// Registering our handler functions, and creating paths.
	http.HandleFunc("/ingest", ingest)

	log.Info().Msgf("Starting on %s:%d", Config.Http.Host, Config.Http.Port)
	fmt.Println("To close connection CTRL+C :-)")
	// Spinning up the server.
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", Config.Http.Host, Config.Http.Port), nil)
	if err != nil {
		log.Fatal().Msgf("Failed to start server: %s", err)
	}
}
