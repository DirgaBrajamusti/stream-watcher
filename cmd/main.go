package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/webserver"
	"streamwatcher/provider/rplay"
	"streamwatcher/provider/twitch"
	"streamwatcher/provider/youtube"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))

	if err := config.LoadConfig(); err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := common.MkdirAll("temp", "downloads"); err != nil {
		slog.Error("failed to initialize directories", "error", err)
		os.Exit(1)
	}

	slog.Info("starting stream watcher")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := webserver.NewServer()
	server.Start()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scheduler(ctx)
	}()

	// Run an initial check immediately.
	checkAll(ctx)

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("shutting down")
	cancel()

	// Give active downloads a grace period to clean up.
	wg.Wait()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Stop(shutdownCtx); err != nil {
		slog.Error("failed to stop web server gracefully", "error", err)
	}
}

func scheduler(ctx context.Context) {
	ticker := time.NewTicker(config.CheckInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkAll(ctx)
		}
	}
}

func checkAll(ctx context.Context) {
	cfg := config.Get().Archive
	if cfg.YouTube {
		youtube.CheckLiveAllChannel(ctx)
	}
	if cfg.Twitch {
		twitch.CheckLiveAllChannel(ctx)
	}
	if cfg.RPLAY {
		rplay.CheckLiveAllChannel(ctx)
	}
}
