package obd2influx

import (
	"database/sql"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func ConnectPg() {
	var err error
	db, err = sql.Open("postgres", K.MustString("pg.connection"))
	if err != nil {
		log.Fatal().Err(err)
	}
}

func getOwnerFromMail(mail string) (string, error) {
	var owner string
	err := db.QueryRow("SELECT name FROM owners WHERE mail = $1", mail).Scan(&owner)
	if err != nil {
		log.Err(err).Send()
		return "", err
	}
	if *Config.DryRun {
		log.Info().Msgf("Owner %s found", owner)
	}
	return owner, nil
}

func registerTrip(vehicle Vehicle, owner string, start time.Time, end time.Time) string {
	// Check if user exists
	var userId int
	bucket := strings.ToLower(owner)
	if *Config.DryRun {
		log.Info().Msgf("Dry run, not registering trip, bucket: %s", bucket)
		return bucket
	}

	err := db.QueryRow("SELECT owner_id, bucket FROM owners WHERE name = $1", owner).Scan(&userId, &bucket)
	if err != nil {
		log.Err(err).Send()
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
