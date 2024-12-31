package main

import (
	"flag"
	"streamwatcher/helpers/webserver"
	"streamwatcher/provider/twitch"
	"streamwatcher/provider/youtube"
	"time"

	"streamwatcher/config"

	"github.com/kataras/golog"
)

func archivers() {
	if config.AppConfig.Archive.YouTube {
		youtube.CheckLiveAllChannel()
	}
	if config.AppConfig.Archive.Twitch {
		twitch.CheckLiveAllChannel()
	}
}

func main() {
	config.LoadConfig()
	go webserver.StartServer()
	debug := flag.Bool("debug", false, "enable debug mode")

	// Parse flags
	flag.Parse()

	// Use the debug flag
	if *debug {
		golog.SetLevel("debug")
	}

	golog.Info("Starting...")
	archivers()

	ticker := time.NewTicker(time.Duration(config.AppConfig.Archive.Checker) * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			archivers()
		}
	}
}
