package discord

import (
	"bytes"
	"encoding/json"
	"net/http"
	"streamwatcher/config"

	"github.com/kataras/golog"
)

type Author struct {
	Name    string      `json:"name"`
	URL     string      `json:"url"`
	IconURL interface{} `json:"icon_url"`
}

type Footer struct {
	Text string `json:"text"`
}

type Thumbnail struct {
	URL string `json:"url"`
}

type Embed struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Color       int       `json:"color"`
	Author      Author    `json:"author"`
	Footer      Footer    `json:"footer"`
	Thumbnail   Thumbnail `json:"thumbnail"`
}

type DiscordPayload struct {
	Content     interface{} `json:"content"`
	Embeds      []Embed     `json:"embeds"`
	Username    string      `json:"username"`
	Attachments []string    `json:"attachments"`
}

func SendNotificationWebhook(channelName string, title string, videoUrl string, thumbnailUrl string, status string) {
	color := map[string]int{
		"Recording": 65280,
		"Done":      9934835,
		"Error":     16711680,
	}

	if config.AppConfig.Discord.Notify {
		headers := map[string]string{
			"Content-Type": "application/json",
		}
		payload := DiscordPayload{
			Content: nil,
			Embeds: []Embed{
				{
					Title:       status,
					Description: title,
					Color:       color[status],
					Author: Author{
						Name:    channelName + " is live!",
						URL:     videoUrl,
						IconURL: nil,
					},
					Footer: Footer{
						Text: "Shiodome v0.0.1",
					},
					Thumbnail: Thumbnail{
						URL: thumbnailUrl,
					},
				},
			},
			Username:    "Shiodome",
			Attachments: []string{},
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return
		}

		client := &http.Client{}
		req, err := http.NewRequest("POST", config.AppConfig.Discord.Webhook, bytes.NewBuffer(jsonPayload))
		if err != nil {
			golog.Error("discord send notification error:", err)
			return
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			golog.Error("[discord] error sending request:", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			golog.Debug("discord send notification successfully")
		} else {
			golog.Error("discord send notification error")
		}

	}
}
