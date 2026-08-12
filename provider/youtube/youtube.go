package youtube

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"streamwatcher/helpers/ytarchive"
)

const (
	ytBaseURL = "https://www.youtube.com"
)

var (
	reLive         = regexp.MustCompile(`"text\+":"LIVE"`)
	reVideoID      = regexp.MustCompile(`"videoId":"([^"]+)`)
	reMembersOnly  = regexp.MustCompile(`"[a-zA-Z]+":"Members only"`)
	reTitle        = regexp.MustCompile(`title\":{\"runs\":\[\{\"text\":\"([^\"]+)`)
	reChannelPic   = regexp.MustCompile(`<meta name="twitter:image" content="(.*?)">`)
	reChannelName  = regexp.MustCompile(`<meta\s+property="og:title"\s+content="([^"]+)">`)
	reVideoTitle   = regexp.MustCompile(`"videoDetails":\{"videoId":"[^"]+","title":"([^"]+)"`)
	reVideoChannel = regexp.MustCompile(`"videoDetails":\{[^}]*"channelId":"([^"]+)"`)
	reChannelName2 = regexp.MustCompile(`ChannelName":"(.*?)"`)
)

var channelErrorTracker = common.NewErrorTracker()

// netscapeCookie represents a single line in a Netscape cookie file.
type netscapeCookie struct {
	Domain     string
	Flag       string
	Path       string
	Secure     bool
	Expiration int64
	Name       string
	Value      string
}

// ParseNetscapeCookieFile reads cookies from a Netscape-format cookie file.
func ParseNetscapeCookieFile(filepath string) ([]*http.Cookie, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("read cookie file: %w", err)
	}

	var cookies []*http.Cookie
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		expires := int64(0)
		if _, err := fmt.Sscanf(fields[4], "%d", &expires); err != nil {
			expires = 0
		}

		cookies = append(cookies, &http.Cookie{
			Domain:   fields[0],
			Path:     fields[2],
			Secure:   fields[3] == "TRUE",
			Expires:  time.Unix(expires, 0),
			Name:     fields[5],
			Value:    fields[6],
			HttpOnly: true,
		})
	}
	return cookies, nil
}

// httpClientWithCookies returns an HTTP client that includes the configured cookies.
func httpClientWithCookies(useMemberCookies bool) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	cfg := config.Get().Archive
	cookiePath := cfg.Cookies
	if useMemberCookies && cfg.MemberCookies != "" {
		cookiePath = cfg.MemberCookies
	}

	if cookiePath != "" {
		cookies, err := ParseNetscapeCookieFile(cookiePath)
		if err != nil {
			return nil, err
		}
		u, _ := url.Parse(ytBaseURL)
		jar.SetCookies(u, cookies)
	}

	return &http.Client{Jar: jar, Timeout: 30 * time.Second}, nil
}

// GetChannelLive checks a YouTube channel's streams page and returns the first live
// stream that matches the channel configuration. When the request keeps failing, a
// Discord notification is sent once the consecutive failure count reaches
// common.ErrorThreshold.
func GetChannelLive(ctx context.Context, channelID string, useMemberCookies bool) (*common.ChannelLive, error) {
	live, err := getChannelLive(ctx, channelID, useMemberCookies)
	if err != nil {
		if channelErrorTracker.Fail("youtube:" + channelID) {
			discord.SendNotification(ctx, youtubeChannelName(channelID), fmt.Sprintf("YouTube API error: %v", err),
				fmt.Sprintf("https://www.youtube.com/channel/%s", channelID), "", "Error")
		}
	} else {
		channelErrorTracker.Success("youtube:" + channelID)
	}
	return live, err
}

func getChannelLive(ctx context.Context, channelID string, useMemberCookies bool) (*common.ChannelLive, error) {
	client, err := httpClientWithCookies(useMemberCookies)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/channel/%s/streams", ytBaseURL, channelID), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch channel streams: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	fragments := strings.Split(string(body), "videoRenderer")
	for _, fragment := range fragments[1:] {
		if !reLive.MatchString(fragment) {
			continue
		}

		videoID := firstMatch(reVideoID, fragment)
		if videoID == "" {
			continue
		}
		title := firstMatch(reTitle, fragment)
		if title == "" {
			continue
		}

		channelName := firstMatch(reChannelName, string(body))
		channelPic := firstMatch(reChannelPic, string(body))
		isMembersOnly := reMembersOnly.MatchString(fragment)
		dateCrawled := time.Now().UTC()

		return &common.ChannelLive{
			Title:          title,
			ChannelID:      channelID,
			ThumbnailURL:   fmt.Sprintf("https://img.youtube.com/vi/%s/0.jpg", videoID),
			VideoID:        videoID,
			ChannelName:    channelName,
			ChannelPicture: channelPic,
			DateCrawled:    dateCrawled,
			MembersOnly:    isMembersOnly,
			Platform:       "youtube",
		}, nil
	}
	return nil, nil
}

// GetVideoDetailsFromID fetches video details from a YouTube video ID.
func GetVideoDetailsFromID(ctx context.Context, videoID string) (*common.ChannelLive, error) {
	client, err := httpClientWithCookies(false)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/watch?v=%s", ytBaseURL, videoID), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch video details: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	title := firstMatch(reVideoTitle, string(body))
	if title == "" {
		return nil, fmt.Errorf("no title found for video %s", videoID)
	}
	channelID := firstMatch(reVideoChannel, string(body))
	if channelID == "" {
		return nil, fmt.Errorf("no channel id found for video %s", videoID)
	}
	channelPic := firstMatch(reChannelPic, string(body))
	channelName := firstMatch(reChannelName2, string(body))

	return &common.ChannelLive{
		Title:          title,
		ChannelID:      channelID,
		ThumbnailURL:   fmt.Sprintf("https://img.youtube.com/vi/%s/0.jpg", videoID),
		VideoID:        videoID,
		ChannelName:    channelName,
		ChannelPicture: channelPic,
		DateCrawled:    time.Now().UTC(),
		Platform:       "youtube",
	}, nil
}

// CheckLiveAllChannel checks every configured YouTube channel for live streams.
func CheckLiveAllChannel(ctx context.Context) {
	cfg := config.Get()
	for i, channel := range cfg.YouTubeChannel {
		slog.Info("youtube: checking channel", "channel", channel.Name)
		live, err := GetChannelLive(ctx, channel.ID, channel.UseMemberCookies)
		if err != nil {
			slog.Error("youtube: failed to fetch channel", "channel", channel.Name, "error", err)
			continue
		}
		if live == nil {
			slog.Debug("youtube: channel is not live", "channel", channel.Name)
			continue
		}

		if shouldDownload(live, channel) {
			slog.Info("youtube: live stream detected", "channel", channel.Name, "title", live.Title)
			discord.SendNotification(ctx, live.ChannelName, live.Title,
				fmt.Sprintf("https://www.youtube.com/watch?v=%s", live.VideoID), live.ThumbnailURL, "Recording")
			go ytarchive.StartDownload(ctx,
				fmt.Sprintf("https://www.youtube.com/watch?v=%s", live.VideoID),
				[]string{}, live, channel.OutPath)
		}

		if i < len(cfg.YouTubeChannel)-1 {
			select {
			case <-time.After(config.CheckInterval()):
			case <-ctx.Done():
				return
			}
		}
	}
}

// shouldDownload applies channel rules to decide whether to record the live stream.
func shouldDownload(live *common.ChannelLive, channel config.YouTubeChannel) bool {
	if live == nil {
		return false
	}
	if common.DefaultStore.Exists(live.VideoID) {
		slog.Debug("youtube: video already in download jobs", "video_id", live.VideoID)
		return false
	}
	if live.MembersOnly && !channel.UseMemberCookies {
		slog.Debug("youtube: members-only stream without member cookies", "video_id", live.VideoID)
		return false
	}
	if common.MatchFilters(live.Title, channel.Filters) {
		return true
	}
	if live.MembersOnly && channel.AlwaysDownloadMember {
		return true
	}
	slog.Debug("youtube: stream does not match filters", "title", live.Title)
	return false
}

// ParseVideoID extracts a YouTube video ID from a URL.
func ParseVideoID(parsed *url.URL) (string, bool) {
	host := parsed.Host
	path := parsed.Path

	switch {
	case host == "youtu.be":
		id := strings.TrimPrefix(path, "/")
		return id, id != ""
	case strings.HasPrefix(path, "/live/"):
		id := strings.TrimPrefix(path, "/live/")
		return id, id != ""
	case path == "/watch":
		id := parsed.Query().Get("v")
		return id, id != ""
	}
	return "", false
}

// youtubeChannelName resolves a channel display name from the configured YouTube
// channels, falling back to the channel ID when it is not configured.
func youtubeChannelName(channelID string) string {
	for _, ch := range config.Get().YouTubeChannel {
		if ch.ID == channelID {
			return ch.Name
		}
	}
	return channelID
}

func firstMatch(re *regexp.Regexp, text string) string {
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}
