package main

import (
	"flag"
	"os"

	obd2influx "github.com/Obito1903/obd2influx/pkg"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	serve := flag.Bool("serve", false, "Start the HTTP server")
	obd2influx.Config.ListPids = flag.Bool("list-pids", false, "List all PIDs")
	obd2influx.Config.DryRun = flag.Bool("dry-run", false, "Dry run")

	flag.Parse()
	log.Debug().Msgf("Config: %+v", obd2influx.Config)

	obd2influx.InitWithConfigFile(*configFile)
	if *serve {
		obd2influx.Serve()
	} else {
		obd2influx.ProcessFolders()
	}
}
