package youtube

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"streamwatcher/helpers/ytarchive"

	"strings"
	"time"

	"github.com/kataras/golog"
)

// NetscapeCookie represents a cookie in Netscape format
type NetscapeCookie struct {
	Domain     string
	Flag       string
	Path       string
	Secure     bool
	Expiration int64
	Name       string
	Value      string
}

// ParseNetscapeCookieFile reads and parses cookies from a Netscape format cookie file
func ParseNetscapeCookieFile(filepath string) ([]*http.Cookie, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cookie file: %v", err)
	}

	var cookies []*http.Cookie
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
			continue
		}

		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) < 7 {
			continue
		}

		expiration, err := time.Parse("2006", fields[4])
		if err != nil {
			expireInt := int64(0)
			fmt.Sscanf(fields[4], "%d", &expireInt)
			expiration = time.Unix(expireInt, 0)
		}

		cookie := &http.Cookie{
			Domain:   fields[0],
			Path:     fields[2],
			Secure:   fields[3] == "TRUE",
			Expires:  expiration,
			Name:     fields[5],
			Value:    fields[6],
			HttpOnly: true,
		}
		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

func GetChannelLive(channelID string) (*common.ChannelLive, error) {
	// Create a cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %v", err)
	}

	// Parse and set cookies if file path is provided
	var cookieFilePath string
	if config.AppConfig.Archive.Cookies != "" {
		cookieFilePath = config.AppConfig.Archive.Cookies
	} else {
		cookieFilePath = ""
	}
	if cookieFilePath != "" {
		cookies, err := ParseNetscapeCookieFile(cookieFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cookie file: %v", err)
		}

		ytUrl, _ := url.Parse("https://www.youtube.com")
		jar.SetCookies(ytUrl, cookies)
	}

	// Create HTTP client with cookie jar
	client := &http.Client{
		Jar: jar,
	}

	// Make request with cookies
	// resp, err := client.Get(fmt.Sprintf("https://www.youtube.com/channel/%s/live", channelID))
	resp, err := client.Get(fmt.Sprintf("https://www.youtube.com/channel/%s/streams", channelID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	fragments := strings.Split(string(body), "videoRenderer")
	for _, fragment := range fragments {
		reLive := regexp.MustCompile(`"text+":"LIVE"`)
		if reLive.MatchString(fragment) {
			reVideoID := regexp.MustCompile(`"videoId":"([^"]+)`)
			videoID := reVideoID.FindStringSubmatch(fragment)
			if len(videoID) < 2 {
				return nil, fmt.Errorf("no video id found")
			}
			reTitle := regexp.MustCompile(`title":{"runs":\[{"text":"([^"]+)`)
			title := reTitle.FindStringSubmatch(fragment)
			if len(title) < 2 {
				return nil, fmt.Errorf("no title found")
			}

			reChannelPic := regexp.MustCompile(`<meta name="twitter:image" content="(.*?)"`)
			channelPic := reChannelPic.FindStringSubmatch(string(body))
			if len(channelPic) < 2 {
				return nil, fmt.Errorf("no channel pic found")
			}

			reChannelName := regexp.MustCompile(`<meta\s+property="og:title"\s+content="([^"]+)">`)
			channelName := reChannelName.FindStringSubmatch(string(body))
			if len(channelName) < 2 {
				return nil, fmt.Errorf("no channel name found")
			}

			dateCrawled := time.Now().UTC().Format(time.RFC3339Nano)
			return &common.ChannelLive{
				Title:          title[1],
				ChannelID:      channelID,
				ThumbnailUrl:   fmt.Sprintf("https://img.youtube.com/vi/%s/0.jpg", videoID[1]),
				VideoID:        videoID[1],
				ChannelName:    channelName[1],
				ChannelPicture: channelPic[1],
				DateCrawled:    dateCrawled,
			}, nil
		}
	}
	return nil, nil
}

func GetVideoDetailsFromID(videoID string) (*common.ChannelLive, error) {
	// Create a cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %v", err)
	}

	// Parse and set cookies if file path is provided
	var cookieFilePath string
	if config.AppConfig.Archive.Cookies != "" {
		cookieFilePath = config.AppConfig.Archive.Cookies
	} else {
		cookieFilePath = ""
	}
	if cookieFilePath != "" {
		cookies, err := ParseNetscapeCookieFile(cookieFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cookie file: %v", err)
		}

		ytUrl, _ := url.Parse("https://www.youtube.com")
		jar.SetCookies(ytUrl, cookies)
	}

	// Create HTTP client with cookie jar
	client := &http.Client{
		Jar: jar,
	}

	// Make request with cookies
	resp, err := client.Get(fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	reTitle := regexp.MustCompile(`"videoDetails":\{"videoId":"[^"]+","title":"([^"]+)"`)
	title := reTitle.FindStringSubmatch(string(body))
	if len(title) < 2 {
		return nil, fmt.Errorf("no title found")
	}

	reChannelId := regexp.MustCompile(`"videoDetails":\{[^}]*"channelId":"([^"]+)"`)
	channelId := reChannelId.FindStringSubmatch(string(body))
	if len(channelId) < 2 {
		return nil, fmt.Errorf("no channel id found")
	}

	reChannelPic := regexp.MustCompile(`<meta name="twitter:image" content="(.*?)"`)
	channelPic := reChannelPic.FindStringSubmatch(string(body))

	if len(channelPic) < 2 {
		return nil, fmt.Errorf("no channel pic found")
	}

	reChannelName := regexp.MustCompile(`ChannelName":"(.*?)"`)
	channelName := reChannelName.FindStringSubmatch(string(body))

	if len(channelName) < 2 {
		return nil, fmt.Errorf("no channel name found")
	}

	dateCrawled := time.Now().UTC().Format(time.RFC3339Nano)

	return &common.ChannelLive{
		Title:          title[1],
		ChannelID:      channelId[1],
		ThumbnailUrl:   fmt.Sprintf("https://img.youtube.com/vi/%s/0.jpg", videoID),
		VideoID:        videoID,
		ChannelName:    channelName[1],
		ChannelPicture: channelPic[1],
		DateCrawled:    dateCrawled,
	}, nil
}

func CheckLiveAllChannel() {
	for i, channel := range config.AppConfig.YouTubeChannel {
		golog.Info("[youtube] checking live: ", channel.Name)
		channelLive, err := GetChannelLive(channel.ID)
		if err != nil {
			golog.Error(err)
		}
		if channelLive != nil {
			if common.IsVideoIDInDownloadJobs(channelLive.VideoID) {
				golog.Debug("[youtube] live is in download jobs: ", channel.Name)
			} else {
				videoInRegex := common.CheckVideoRegex(channelLive.Title, channel.Filters)
				if videoInRegex {
					golog.Info("[youtube] live is in regex: ", channel.Name)
					discord.SendNotificationWebhook(channelLive.ChannelName, channelLive.Title, "https://www.youtube.com/watch?v="+channelLive.VideoID, channelLive.ThumbnailUrl, "Recording")
					go func() {
						ytarchive.StartDownload("https://www.youtube.com/watch?v="+channelLive.VideoID, []string{}, channelLive, channel.OutPath)
					}()
				} else {
					golog.Debug("[youtube] live is not in regex: ", channel.Name)
				}
			}
		} else {
			golog.Debug("[youtube] channel is not live: ", channel.Name)
		}
		if i < len(config.AppConfig.YouTubeChannel)-1 {
			time.Sleep(time.Duration(config.AppConfig.Archive.Checker) * time.Minute)
		}
	}
}

func ParseVideoID(parsedURI *url.URL) *string {
	host := parsedURI.Host
	path := parsedURI.Path

	if host == "youtu.be" {
		id := path[1:]
		return &id
	}

	if strings.HasPrefix(path, "/live/") {
		id := path[6:]
		return &id
	}

	if path == "/watch" {
		query := parsedURI.Query()
		if id, ok := query["v"]; ok && len(id) > 0 {
			return &id[0]
		}
	}

	return nil
}
