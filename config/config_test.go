package config

import (
	"testing"
)

func TestValidateValid(t *testing.T) {
	cfg := &Config{
		Archive:   ArchiveConfig{Checker: 1},
		Webserver: WebserverConfig{Host: "0.0.0.0", Port: "3000"},
		YouTubeChannel: []YouTubeChannel{
			{ID: "UC123", Name: "Test", OutPath: "./downloads/test"},
		},
		Discord: DiscordConfig{Notify: false},
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
}

func TestValidateDiscordWithoutWebhook(t *testing.T) {
	cfg := &Config{
		Archive:   ArchiveConfig{Checker: 1},
		Webserver: WebserverConfig{Port: "3000"},
		Discord:   DiscordConfig{Notify: true, Webhook: ""},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected validation error for discord webhook")
	}
}

func TestValidateMissingYouTubeID(t *testing.T) {
	cfg := &Config{
		Archive:   ArchiveConfig{Checker: 1},
		Webserver: WebserverConfig{Port: "3000"},
		YouTubeChannel: []YouTubeChannel{
			{OutPath: "./downloads/test"},
		},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected validation error for missing youtube id")
	}
}
