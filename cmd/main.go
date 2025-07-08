package main

import (
	"flag"
	"os"
	"streamwatcher/helpers/webserver"
	"streamwatcher/provider/twitch"
	"streamwatcher/provider/youtube"
	"sync"
	"time"

	"streamwatcher/config"

	"github.com/kataras/golog"
)

var (
	twitchMutex  sync.Mutex
	youtubeMutex sync.Mutex
)

func safeTwitchCheck() {
	if !twitchMutex.TryLock() {
		golog.Debug("[System] Twitch checker is already running")
		return
	}
	defer twitchMutex.Unlock()
	golog.Debug("[System] Running Twitch check")
	twitch.CheckLiveAllChannel()
}

func safeYouTubeCheck() {
	if !youtubeMutex.TryLock() {
		golog.Debug("[System] YouTube checker is already running")
		return
	}
	defer youtubeMutex.Unlock()
	golog.Debug("[System] Running YouTube check")
	youtube.CheckLiveAllChannel()
}

func archivers() {
	golog.Debug("[System] Scheduled check for live channels")
	if config.AppConfig.Archive.YouTube {
		safeYouTubeCheck()
	}
	if config.AppConfig.Archive.Twitch {
		safeTwitchCheck()
	}
}

func initialized() {
	golog.Debug("[System] Creating directories if not exists")
	createDirIfNotExist("temp")
	createDirIfNotExist("downloads")
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
	flag.Parse()
	if *debug {
		golog.SetLevel("debug")
	}
	golog.Infof("[System] Starting...")
	initialized()

	archivers() // Initial check at startup

	ticker := time.NewTicker(time.Duration(config.AppConfig.Archive.Checker) * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		archivers()
	}
}
