// config/config.go
package config

import (
	"github.com/fsnotify/fsnotify"
	"github.com/kataras/golog"
	"github.com/spf13/viper"
)

// Structs to hold application configuration
type YTDLPConfig struct {
	ExecutablePath   string   `mapstructure:"executable_path"`
	WorkingDirectory string   `mapstructure:"working_directory"`
	Args             []string `mapstructure:"args"`
}

type YTArchive struct {
	ExecutablePath   string   `mapstructure:"executable_path"`
	WorkingDirectory string   `mapstructure:"working_directory"`
	Args             []string `mapstructure:"args"`
	Quality          string   `mapstructure:"quality"`
	DelayStart       string   `mapstructure:"delay_start"`
	OutPath          string   `mapstructure:"out_path"`
}

type ArchiveConfig struct {
	Cookies string `mapstructure:"cookies"`
	Checker int    `mapstructure:"checker"`
	Twitch  bool   `mapstructure:"twitch"`
	YouTube bool   `mapstructure:"youtube"`
}

type DiscordConfig struct {
	Notify  bool   `mapstructure:"notify"`
	Webhook string `mapstructure:"webhook"`
}

type YouTubeChannel struct {
	ID      string   `mapstructure:"id"`
	Name    string   `mapstructure:"name"`
	Filters []string `mapstructure:"filters"`
	OutPath string   `mapstructure:"out_path"`
}

type TwitchChannel struct {
	Name    string   `mapstructure:"name"`
	Filters []string `mapstructure:"filters"`
	OutPath string   `mapstructure:"out_path"`
}

type WebserverConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

// Main configuration struct
type Config struct {
	YT_DLP         YTDLPConfig      `mapstructure:"yt-dlp"`
	YTArchive      YTArchive        `mapstructure:"ytarchive"`
	Archive        ArchiveConfig    `mapstructure:"archive"`
	Discord        DiscordConfig    `mapstructure:"discord"`
	YouTubeChannel []YouTubeChannel `mapstructure:"youtube_channel"` // Keep as slice
	TwitchChannel  []TwitchChannel  `mapstructure:"twitch_channel"`  // Keep as slice
	Webserver      WebserverConfig  `mapstructure:"webserver"`
}

var AppConfig Config

// LoadConfig reads the config file and populates AppConfig
func LoadConfig() {
	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(".")      // path to look for the config file
	viper.SetConfigType("toml")   // file type

	if err := viper.ReadInConfig(); err != nil {
		golog.Fatal("Error reading config file, ", err)
	}

	// Unmarshal the config into the AppConfig struct
	if err := viper.Unmarshal(&AppConfig); err != nil {
		golog.Fatal("Unable to decode into struct, ", err)
	}

	viper.OnConfigChange(func(e fsnotify.Event) {
		golog.Info("Config file changed")
		if err := viper.Unmarshal(&AppConfig); err != nil {
			golog.Error("Error reloading config:", err)
		}
	})
	viper.WatchConfig()
}
