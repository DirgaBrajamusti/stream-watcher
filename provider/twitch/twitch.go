package twitch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"streamwatcher/helpers/streamlink"
	"streamwatcher/helpers/ytdlp"
	"time"

	"github.com/kataras/golog"
)

var IsCheckingInProgress bool

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
	streamProfilePic := resData.Data.User.ProfileImageURL
	DateCrawled := time.Now().UTC().Format(time.RFC3339Nano)

	if streamInfo == nil {
		return nil, nil
	}

	return &common.ChannelLive{
		Title:          streamInfo.Title,
		ChannelID:      username,
		ThumbnailUrl:   streamInfo.PreviewImageURL,
		VideoID:        streamInfo.ID,
		ChannelName:    username,
		ChannelPicture: streamProfilePic,
		DateCrawled:    DateCrawled,
	}, nil
}

func CheckLiveAllChannel() {
	if IsCheckingInProgress {
		return
	}
	IsCheckingInProgress = true
	for i, channel := range config.AppConfig.TwitchChannel {
		if common.IsChannelIDInDownloadJobsAndFinished(channel.Name) {
			golog.Debug("[twitch] ", channel.Name, " is already in download jobs")
			break
		}
		golog.Info("[twitch] Checking if ", channel.Name, " is live")
		channelLive, err := GetChannelInfo(channel.Name)
		if err != nil {
			golog.Error(err)
		}
		if channelLive != nil {
			if common.IsChannelIDInDownloadJobsAndFinished(channelLive.ChannelName) {
				golog.Debug("[twitch] ", channel.Name, " is already in download jobs")
			} else {
				videoInRegex := common.CheckVideoRegex(channelLive.Title, channel.Filters)
				if videoInRegex {
					golog.Info("[twitch] ", channel.Name, " is live: ", channelLive.Title)
					discord.SendNotificationWebhook(channel.Name, channelLive.Title, "https://twitch.tv"+channel.Name, channelLive.ThumbnailUrl, "Recording")
					go func() {
						if config.AppConfig.Archive.TwitchUsingStreamlink {
							streamlink.StartDownload("https://twitch.tv/"+channel.Name, []string{}, channelLive, channel.OutPath)
						} else {
							ytdlp.StartDownload("https://twitch.tv/"+channel.Name, []string{}, channelLive, channel.OutPath)
						}
						// streamlink.StartDownload("https://twitch.tv/"+channel.Name, []string{}, channelLive, channel.OutPath)
					}()
				} else {
					golog.Debug("[twitch] ", channel.Name, " is live but not in filter")
				}
			}
		} else {
			golog.Debug("[twitch] ", channel.Name, " is not live")
		}
		if i < len(config.AppConfig.TwitchChannel)-1 {
			golog.Debug("[twitch] Waiting ", config.AppConfig.Archive.Checker, " minutes before checking next channel")
			time.Sleep(time.Duration(config.AppConfig.Archive.Checker) * time.Minute)
		}
	}
	IsCheckingInProgress = false
}
