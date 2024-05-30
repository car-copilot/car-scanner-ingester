package obd2influx

import "time"

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
