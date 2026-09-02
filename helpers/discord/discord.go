package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"streamwatcher/config"
)

const defaultUsername = "Stream Watcher"

var statusColors = map[string]int{
	"Recording": 65280,    // green
	"Live":      3447003,  // blue
	"Ended":     15108834, // orange
	"Done":      9934835,  // purple-ish
	"Error":     16711680, // red
}

// Embed is a Discord embed object.
type Embed struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Color       int       `json:"color"`
	Author      Author    `json:"author"`
	Footer      Footer    `json:"footer"`
	Thumbnail   Thumbnail `json:"thumbnail"`
}

type Author struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	IconURL string `json:"icon_url,omitempty"`
}

type Footer struct {
	Text string `json:"text"`
}

type Thumbnail struct {
	URL string `json:"url"`
}

type Payload struct {
	Content     string  `json:"content,omitempty"`
	Embeds      []Embed `json:"embeds"`
	Username    string  `json:"username"`
	Attachments []any   `json:"attachments,omitempty"`
}

// SendNotification sends a Discord webhook notification if enabled in the config.
func SendNotification(ctx context.Context, channelName, title, videoURL, thumbnailURL, status string) {
	cfg := config.Get().Discord
	if !cfg.Notify || cfg.Webhook == "" {
		return
	}

	color, ok := statusColors[status]
	if !ok {
		color = statusColors["Recording"]
	}

	payload := Payload{
		Embeds: []Embed{
			{
				Title:       status,
				Description: title,
				Color:       color,
				Author: Author{
					Name: fmt.Sprintf("%s is live!", channelName),
					URL:  videoURL,
				},
				Footer: Footer{
					Text: defaultUsername,
				},
				Thumbnail: Thumbnail{
					URL: thumbnailURL,
				},
			},
		},
		Username: defaultUsername,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("discord: failed to marshal payload", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Webhook, bytes.NewReader(body))
	if err != nil {
		slog.Error("discord: failed to create request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("discord: failed to send webhook", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Debug("discord: notification sent", "status", status, "channel", channelName)
	} else {
		slog.Error("discord: webhook returned non-success status", "status", resp.StatusCode)
	}
}
