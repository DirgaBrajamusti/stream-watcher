package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Defaults for executables and directories.
const (
	DefaultConfigName = "config"
	DefaultConfigType = "toml"
)

// YTDLPConfig holds yt-dlp-specific settings.
type YTDLPConfig struct {
	ExecutablePath   string   `mapstructure:"executable_path"`
	WorkingDirectory string   `mapstructure:"working_directory"`
	Args             []string `mapstructure:"args"`
}

// YTArchiveConfig holds ytarchive-specific settings.
type YTArchiveConfig struct {
	ExecutablePath   string   `mapstructure:"executable_path"`
	WorkingDirectory string   `mapstructure:"working_directory"`
	Args             []string `mapstructure:"args"`
	Quality          string   `mapstructure:"quality"`
	DelayStart       string   `mapstructure:"delay_start"`
}

// StreamlinkConfig holds streamlink-specific settings.
type StreamlinkConfig struct {
	ExecutablePath   string   `mapstructure:"executable_path"`
	WorkingDirectory string   `mapstructure:"working_directory"`
	Args             []string `mapstructure:"args"`
}

// ArchiveConfig holds the global archive settings.
type ArchiveConfig struct {
	Cookies               string `mapstructure:"cookies"`
	MemberCookies         string `mapstructure:"member_cookies"`
	Checker               int    `mapstructure:"checker"`
	Twitch                bool   `mapstructure:"twitch"`
	YouTube               bool   `mapstructure:"youtube"`
	RPLAY                 bool   `mapstructure:"rplay"`
	TwitchUsingStreamlink bool   `mapstructure:"twitch_using_streamlink"`
}

type RPLAYAuthConfig struct {
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password"`
	UserID   string `mapstructure:"user_id"`
}

type RPLAYChannel struct {
	UserID  string `mapstructure:"user_id"`
	Name    string `mapstructure:"name"`
	OutPath string `mapstructure:"out_path"`
}

// DiscordConfig holds the Discord webhook configuration.
type DiscordConfig struct {
	Notify  bool   `mapstructure:"notify"`
	Webhook string `mapstructure:"webhook"`
}

// YouTubeChannel represents a single channel to watch on YouTube.
type YouTubeChannel struct {
	ID                   string   `mapstructure:"id"`
	Name                 string   `mapstructure:"name"`
	Filters              []string `mapstructure:"filters"`
	OutPath              string   `mapstructure:"out_path"`
	AlwaysDownloadMember bool     `mapstructure:"always_download_member"`
	UseMemberCookies     bool     `mapstructure:"use_member_cookies"`
}

// TwitchChannel represents a single channel to watch on Twitch.
type TwitchChannel struct {
	Name    string   `mapstructure:"name"`
	Filters []string `mapstructure:"filters"`
	OutPath string   `mapstructure:"out_path"`
}

// WebserverConfig holds the web server bind settings.
type WebserverConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

// Config is the top-level configuration container.
// Main configuration struct
type Config struct {
	YTDLP          YTDLPConfig      `mapstructure:"yt-dlp"`
	YTArchive      YTArchiveConfig  `mapstructure:"ytarchive"`
	Streamlink     StreamlinkConfig `mapstructure:"streamlink"`
	Archive        ArchiveConfig    `mapstructure:"archive"`
	Discord        DiscordConfig    `mapstructure:"discord"`
	RPLAY          RPLAYAuthConfig  `mapstructure:"rplay"`
	RPLAYChannel   []RPLAYChannel   `mapstructure:"rplay_channel"`
	YouTubeChannel []YouTubeChannel `mapstructure:"youtube_channel"`
	TwitchChannel  []TwitchChannel  `mapstructure:"twitch_channel"`
	Webserver      WebserverConfig  `mapstructure:"webserver"`
}

// AppConfig is the loaded active configuration. It is replaced atomically when the
// configuration file is reloaded.
var AppConfig Config

var (
	configMu   sync.RWMutex
	configPath string
	lastMod    time.Time
)

// LoadConfig reads the configuration file, applies defaults, validates it, and starts
// a background poller that reloads the file when it changes.
func LoadConfig() error {
	v := viper.New()
	v.SetConfigName(DefaultConfigName)
	v.SetConfigType(DefaultConfigType)
	v.AddConfigPath(".")

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	configPath = v.ConfigFileUsed()
	setConfig(cfg)

	if stat, err := os.Stat(configPath); err == nil {
		lastMod = stat.ModTime()
	}

	go pollConfigChanges()
	return nil
}

// Get returns a copy of the active configuration.
func Get() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig
}

// ListenAddr returns the host:port string for the web server.
func ListenAddr() string {
	cfg := Get()
	return fmt.Sprintf("%s:%s", cfg.Webserver.Host, cfg.Webserver.Port)
}

// CheckInterval returns the configured polling interval, clamped to a minimum value.
func CheckInterval() time.Duration {
	cfg := Get()
	minutes := cfg.Archive.Checker
	if minutes <= 0 {
		minutes = 1
	}
	return time.Duration(minutes) * time.Minute
}

// ReloadConfig reloads the configuration file from disk and applies it if validation
// succeeds.
func ReloadConfig() error {
	v := viper.New()
	v.SetConfigFile(configPath)

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return err
	}

	setConfig(cfg)
	return nil
}

func setConfig(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()
	AppConfig = cfg
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("yt-dlp.executable_path", "yt-dlp")
	v.SetDefault("yt-dlp.working_directory", "./temp")
	v.SetDefault("ytarchive.executable_path", "ytarchive")
	v.SetDefault("ytarchive.working_directory", "./temp")
	v.SetDefault("ytarchive.quality", "best")
	v.SetDefault("ytarchive.delay_start", "1s")
	v.SetDefault("streamlink.executable_path", "streamlink")
	v.SetDefault("streamlink.working_directory", "./temp")
	v.SetDefault("archive.cookies", "./cookies.txt")
	v.SetDefault("archive.member_cookies", "./cookies.txt")
	v.SetDefault("archive.checker", 1)
	v.SetDefault("archive.twitch", true)
	v.SetDefault("archive.youtube", true)
	v.SetDefault("archive.rplay", false)
	v.SetDefault("webserver.host", "0.0.0.0")
	v.SetDefault("webserver.port", "3000")
	v.SetDefault("discord.notify", false)
}

func validate(cfg *Config) error {
	var errs []string

	if cfg.Archive.Checker < 0 {
		errs = append(errs, "archive.checker must be non-negative")
	}
	if cfg.Webserver.Port == "" {
		errs = append(errs, "webserver.port is required")
	}
	if cfg.Discord.Notify && cfg.Discord.Webhook == "" {
		errs = append(errs, "discord.webhook is required when discord.notify is true")
	}
	for i, ch := range cfg.YouTubeChannel {
		if ch.ID == "" {
			errs = append(errs, fmt.Sprintf("youtube_channel[%d].id is required", i))
		}
		if ch.OutPath == "" {
			errs = append(errs, fmt.Sprintf("youtube_channel[%d].out_path is required", i))
		}
	}
	for i, ch := range cfg.TwitchChannel {
		if ch.Name == "" {
			errs = append(errs, fmt.Sprintf("twitch_channel[%d].name is required", i))
		}
		if ch.OutPath == "" {
			errs = append(errs, fmt.Sprintf("twitch_channel[%d].out_path is required", i))
		}
	}
	if cfg.Archive.RPLAY {
		if cfg.RPLAY.Email == "" || cfg.RPLAY.Password == "" {
			errs = append(errs, "rplay.email and rplay.password are required when rplay archive is enabled")
		}
		for i, ch := range cfg.RPLAYChannel {
			if ch.UserID == "" {
				errs = append(errs, fmt.Sprintf("rplay_channel[%d].user_id is required", i))
			}
			if ch.OutPath == "" {
				errs = append(errs, fmt.Sprintf("rplay_channel[%d].out_path is required", i))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func pollConfigChanges() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stat, err := os.Stat(configPath)
		if err != nil {
			continue
		}
		if stat.ModTime().After(lastMod) {
			if err := ReloadConfig(); err != nil {
				// Logging is not available here; leave reload errors to be discovered
				// on the next tick or by the caller explicitly.
				continue
			}
			lastMod = stat.ModTime()
		}
	}
}

// ResolvePath returns an absolute path relative to the current working directory.
// It is a convenience wrapper for resolving output and cookie paths.
func ResolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
