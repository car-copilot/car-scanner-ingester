package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rzetterberg/elmobd"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	log.Trace().Msg("this is a debug message")
	dev, err := elmobd.NewDevice("/dev/pts/6", true)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create device")
	}
	version, err := dev.GetVersion()

	if err != nil {
		fmt.Println("Failed to get version", err)
		return
	}
	fmt.Println("Device has version", version)
}
