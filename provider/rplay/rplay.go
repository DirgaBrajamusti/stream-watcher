package rplay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"streamwatcher/helpers/rplaydl"
)

var (
	lastLiveMu sync.RWMutex
	lastLive   = make(map[string]string)
)

const playURL = "https://api.rplay.live/live/play"

// streamInfo is the parsed livestream data from the RPLAY API.
type streamInfo struct {
	StreamState     string `json:"streamState"`
	LivestreamURL   string `json:"livestreamUrl"`
	StreamStartTime string `json:"streamStartTime"`
	Title           string `json:"title"`
	ThumbnailURL    string `json:"thumbnailUrl"`
}

// GetChannelInfo checks the RPLAY API and returns live stream metadata if the user
// is currently broadcasting and the account has access.
func GetChannelInfo(ctx context.Context, userID string) (*common.ChannelLive, *streamInfo, error) {
	auth, err := NewAuth()
	if err != nil {
		return nil, nil, err
	}
	if auth.UserID == "" {
		return nil, nil, fmt.Errorf("rplay user_id is not available; add it to the config")
	}

	url := fmt.Sprintf("%s?creatorOid=%s&platformType=rplay&lang=en&requestorOid=%s&loginType=plax",
		playURL, userID, auth.UserID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range auth.Headers() {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("rplay request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read rplay response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("rplay returned status %d: %s", resp.StatusCode, string(body))
	}

	var data streamInfo
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, nil, fmt.Errorf("decode rplay response: %w", err)
	}

	if data.StreamState == "offline" || data.LivestreamURL == "" {
		return nil, nil, nil
	}

	live := &common.ChannelLive{
		Title:          data.Title,
		ChannelID:      userID,
		ThumbnailURL:   data.ThumbnailURL,
		VideoID:        userID + "_" + data.StreamStartTime,
		ChannelName:    "",
		ChannelPicture: "",
		DateCrawled:    time.Now().UTC(),
		Platform:       "rplay",
	}
	return live, &data, nil
}

// CheckLiveAllChannel checks every configured RPLAY channel and starts a download
// when a live stream is available.
func CheckLiveAllChannel(ctx context.Context) {
	cfg := config.Get()
	if !cfg.Archive.RPLAY {
		return
	}

	for i, channel := range cfg.RPLAYChannel {
		if common.DefaultStore.IsChannelBusy(channel.UserID) {
			slog.Debug("rplay: channel already has active job", "user_id", channel.UserID)
			continue
		}

		slog.Info("rplay: checking channel", "user_id", channel.UserID, "name", channel.Name)
		live, info, err := GetChannelInfo(ctx, channel.UserID)
		if err != nil {
			slog.Error("rplay: failed to fetch channel", "user_id", channel.UserID, "error", err)
			continue
		}
		if live == nil || info == nil {
			slog.Debug("rplay: channel is offline", "user_id", channel.UserID)
			continue
		}

		live.ChannelName = channel.Name

		startTime := info.StreamStartTime
		if isAlreadyRecorded(channel.UserID, startTime) {
			slog.Debug("rplay: stream session already recorded", "user_id", channel.UserID, "started_at", startTime)
			continue
		}

		if common.DefaultStore.Exists(live.VideoID) {
			slog.Debug("rplay: video already in download jobs", "video_id", live.VideoID)
			continue
		}

		if common.DefaultStore.IsChannelBusy(channel.UserID) {
			slog.Debug("rplay: channel already has active job", "user_id", channel.UserID)
			continue
		}

		markRecorded(channel.UserID, startTime)

		slog.Info("rplay: live stream detected", "user_id", channel.UserID, "title", live.Title)
		discord.SendNotification(ctx, channel.Name, live.Title,
			fmt.Sprintf("https://rplay.live/creator/%s", channel.UserID), live.ThumbnailURL, "Recording")
		go rplaydl.StartDownload(ctx, info.LivestreamURL, live, channel.OutPath)

		if i < len(cfg.RPLAYChannel)-1 {
			select {
			case <-time.After(config.CheckInterval()):
			case <-ctx.Done():
				return
			}
		}
	}
}

func isAlreadyRecorded(userID, startTime string) bool {
	lastLiveMu.RLock()
	defer lastLiveMu.RUnlock()
	return lastLive[userID] == startTime
}

func markRecorded(userID, startTime string) {
	lastLiveMu.Lock()
	defer lastLiveMu.Unlock()
	lastLive[userID] = startTime
}
