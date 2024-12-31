package twitch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"streamwatcher/helpers/ytdlp"
	"time"

	"github.com/kataras/golog"
)

type StreamInfo struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PreviewImageURL string `json:"previewImageURL"`
}

type User struct {
	ProfileImageURL string      `json:"profileImageURL"`
	Stream          *StreamInfo `json:"stream"`
}

type ResponseData struct {
	Data struct {
		User User `json:"user"`
	} `json:"data"`
}

func GetChannelInfo(username string) (*common.ChannelLive, error) {
	url := "https://gql.twitch.tv/gql"
	query := fmt.Sprintf(`query {
        user(login: "%s") {
            profileImageURL(width:50)
            stream {
                id
                title
                previewImageURL(height: 720, width:1280)
            }
        }
    }`, username)

	reqBody, err := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": map[string]interface{}{},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("client-id", "kimne78kx3ncx6brgo4mv6wki5h1ko")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var resData ResponseData
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return nil, err
	}

	streamInfo := resData.Data.User.Stream
	// streamProfile := resData.Data.User

	if streamInfo == nil {
		return nil, nil
	}
	return &common.ChannelLive{
		Title:        streamInfo.Title,
		ChannelID:    username,
		ThumbnailUrl: streamInfo.PreviewImageURL,
		VideoID:      streamInfo.ID,
	}, nil
}

func CheckLiveAllChannel() {
	for i, channel := range config.AppConfig.TwitchChannel {
		if common.IsChannelIDInDownloadJobs(channel.Name) {
			golog.Debug("[Twitch] ", channel.Name, " is already in download jobs")
			break
		}
		golog.Info("[Twitch] Checking if ", channel.Name, " is live")
		channelLive, err := GetChannelInfo(channel.Name)
		if err != nil {
			golog.Error(err)
		}
		if channelLive != nil {
			if common.IsVideoIDInDownloadJobs(channelLive.VideoID) {
				golog.Debug("[Twitch] ", channel.Name, " is already in download jobs")
			} else {
				videoInRegex := common.CheckVideoRegex(channelLive.Title, channel.Filters)
				if videoInRegex {
					golog.Info("[Twitch] ", channel.Name, " is live: ", channelLive.Title)
					discord.SendNotificationWebhook(channel.Name, channelLive.Title, "https://twitch.tv"+channel.Name, channelLive.ThumbnailUrl, "Recording")
					go func() {
						ytdlp.StartDownload("https://twitch.tv/"+channel.Name, []string{}, channelLive, channel.OutPath)
					}()
				} else {
					golog.Debug("[Twitch] ", channel.Name, " is live but not in filter")
				}
			}
		} else {
			golog.Debug("[Twitch] ", channel.Name, " is not live")
		}
		if i < len(config.AppConfig.TwitchChannel)-1 {
			golog.Debug("[Twitch] Waiting ", config.AppConfig.Archive.Checker, " minutes before checking next channel")
			time.Sleep(time.Duration(config.AppConfig.Archive.Checker) * time.Minute)
		}
	}
}