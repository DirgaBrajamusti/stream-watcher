package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"streamwatcher/helpers/streamlink"
	"streamwatcher/helpers/ytdlp"
)

const (
	gqlURL   = "https://gql.twitch.tv/gql"
	clientID = "kimne78kx3ncx6brgo4mv6wki5h1ko"
)

var (
	channelErrorTracker = common.NewErrorTracker()
)

type streamInfo struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PreviewImageURL string `json:"previewImageURL"`
}

type user struct {
	ProfileImageURL string      `json:"profileImageURL"`
	Stream          *streamInfo `json:"stream"`
}

type responseData struct {
	Data struct {
		User user `json:"user"`
	} `json:"data"`
}

// GetChannelInfo fetches Twitch channel information and returns the live stream if
// the channel is currently broadcasting. When the request keeps failing, a Discord
// notification is sent once the consecutive failure count reaches
// common.ErrorThreshold.
func GetChannelInfo(ctx context.Context, username string) (*common.ChannelLive, error) {
	live, err := getChannelInfo(ctx, username)
	if err != nil {
		if channelErrorTracker.Fail("twitch:" + username) {
			discord.SendNotification(ctx, username, fmt.Sprintf("Twitch API error: %v", err),
				fmt.Sprintf("https://twitch.tv/%s", username), "", "Error")
		}
	} else {
		channelErrorTracker.Success("twitch:" + username)
	}
	return live, err
}

func getChannelInfo(ctx context.Context, username string) (*common.ChannelLive, error) {
	query := fmt.Sprintf(`query {
		user(login: "%s") {
			profileImageURL(width: 50)
			stream {
				id
				title
				previewImageURL(height: 720, width: 1280)
			}
		}
	}`, username)

	reqBody, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": map[string]any{},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("client-id", clientID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch returned status %d", resp.StatusCode)
	}

	var data responseData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if data.Data.User.Stream == nil {
		return nil, nil
	}

	stream := data.Data.User.Stream
	streamProfilePic := data.Data.User.ProfileImageURL
	dateCrawled := time.Now().UTC()

	return &common.ChannelLive{
		Title:          stream.Title,
		ChannelID:      username,
		ThumbnailURL:   stream.PreviewImageURL,
		VideoID:        stream.ID,
		ChannelName:    username,
		ChannelPicture: streamProfilePic,
		DateCrawled:    dateCrawled,
		Platform:       "twitch",
	}, nil
}

// CheckLiveAllChannel checks every configured Twitch channel and starts a download
// when a matching live stream is found.
func CheckLiveAllChannel(ctx context.Context) {
	cfg := config.Get()
	for i, channel := range cfg.TwitchChannel {
		// Throttle API requests: wait between every channel check, including
		// offline or errored channels.
		if i > 0 {
			sleepCtx(ctx, config.TwitchDelay())
		}

		if common.DefaultStore.IsChannelBusy(channel.Name) {
			slog.Debug("twitch: channel already has active job", "channel", channel.Name)
			continue
		}

		slog.Info("twitch: checking channel", "channel", channel.Name)
		live, err := GetChannelInfo(ctx, channel.Name)
		if err != nil {
			slog.Error("twitch: failed to fetch channel", "channel", channel.Name, "error", err)
			continue
		}
		if live == nil {
			slog.Debug("twitch: channel is offline", "channel", channel.Name)
			continue
		}

		if common.DefaultStore.IsChannelBusy(live.ChannelName) {
			slog.Debug("twitch: channel already has active job", "channel", live.ChannelName)
			continue
		}

		if !common.MatchFilters(live.Title, channel.Filters) {
			slog.Debug("twitch: stream does not match filters", "channel", channel.Name, "title", live.Title)
			continue
		}

		slog.Info("twitch: live stream detected", "channel", channel.Name, "title", live.Title)
		discord.SendNotification(ctx, channel.Name, live.Title,
			fmt.Sprintf("https://twitch.tv/%s", channel.Name), live.ThumbnailURL, "Recording")

		outPath := channel.OutPath
		go StartDownload(ctx, channel.Name, live, outPath)
	}
}

// StartDownload selects the configured Twitch downloader and starts it.
func StartDownload(ctx context.Context, username string, live *common.ChannelLive, outPath string) {
	url := fmt.Sprintf("https://twitch.tv/%s", username)
	if config.Get().Archive.TwitchUsingStreamlink {
		streamlink.StartDownload(ctx, url, []string{}, live, outPath)
	} else {
		ytdlp.StartDownload(ctx, url, []string{}, live, outPath)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
