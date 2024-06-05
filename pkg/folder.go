package obd2influx

import (
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func ProcessFolders() {
	// Read List folders in path
	folders, err := os.ReadDir(K.String("path"))
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
		if _, ok := Config.Owners[owner]; !ok {
			Config.Owners[owner] = make(Vehicles, 0)
		}

		files, err := os.ReadDir(path.Join(K.String("path"), folder.Name()))
		if err != nil {
			log.Fatal().Err(err)
		}

		for _, file := range files {
			if !file.IsDir() {
				log.Warn().Msgf("Skipping %s, not a directory", file.Name())
				continue
			}
			// Parse vehicle from folder name
			matches := VehicleRegex.FindStringSubmatch(file.Name())
			if matches == nil {
				log.Warn().Msgf("Skipping %s, not a valid folder name", file.Name())
				continue
			}
			// Parse year
			year, err := strconv.Atoi(matches[6])
			if err != nil {
				log.Warn().Err(err)
			}

			vehicle := Vehicle{
				Brand:  strings.TrimSpace(matches[1]),
				Model:  strings.TrimSpace(matches[2]),
				Engine: strings.TrimSpace(matches[4]),
				Year:   year,
				Path:   path.Join(K.String("path"), folder.Name(), file.Name()),
			}

			log.Debug().Msgf("Found vehicle %s %s %d %s for %s in %s", vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.Engine, owner, vehicle.Path)

			vehicle = FindBestVehicleMatch(Config.Vehicles, vehicle)

			Config.Owners[owner] = append(Config.Owners[owner], vehicle)
		}
	}

	for owner, vehicles := range Config.Owners {
		for _, vehicle := range vehicles {
			interval := vehicle.Interval
			mergedPids := MergeMaps(K.MustStringMap("pidMap"), vehicle.PidMap)
			mergedIgnorePids := MergeSlicesUnique(K.MustStrings("ignorePids"), vehicle.IgnorePids)

			log.Debug().Msgf("vehicle Path: %s", vehicle.Path)
			files, err := os.ReadDir(vehicle.Path)
			if err != nil {
				log.Fatal().Err(err)
			}

			log.Info().Msgf("Processing vehicle %s %s for %s", vehicle.Brand, vehicle.Model, owner)
			for _, file := range files {
				log.Debug().Msgf("Processing file %s", file.Name())
				ProcessFile(file, vehicle, mergedPids, mergedIgnorePids, owner, interval)
			}
		}
	}
}
