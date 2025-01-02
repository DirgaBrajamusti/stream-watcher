package main

import (
	"flag"
	"os"
	"streamwatcher/helpers/webserver"
	"streamwatcher/provider/twitch"
	"streamwatcher/provider/youtube"
	"time"

	"streamwatcher/config"

	"github.com/kataras/golog"
)

func archivers() {
	golog.Debug("[System] Checking for live channels")
	if config.AppConfig.Archive.YouTube {
		golog.Debug("[System] Checking for live Youtube Channels")
		youtube.CheckLiveAllChannel()
	}
	if config.AppConfig.Archive.Twitch {
		golog.Debug("[System] Checking for live Twitch Channels")
		twitch.CheckLiveAllChannel()
	}
}

func initialized() {
	golog.Info("Initializing...")
	createDirIfNotExist("temp")
	createDirIfNotExist("downloads")
	golog.Info("Initialized")
}

func createDirIfNotExist(dir string) {
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		golog.Fatal("Failed to create directory: ", err)
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
	initialized()
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
