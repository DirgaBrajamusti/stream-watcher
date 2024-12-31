package common

import (
	"sync"
)

type Task struct {
	Title           string `json:"title"`
	VideoID         string `json:"video_id"`
	VideoPicture    string `json:"video_picture"`
	ChannelName     string `json:"channel_name"`
	ChannelID       string `json:"channel_id"`
	ChannelPicture  string `json:"channel_picture"`
	OutputDirectory string `json:"output_directory"`
}

type Status struct {
	Version      string  `json:"version"`
	State        any     `json:"state"` // Use `any` since the state can be a string or an object
	LastOutput   *string `json:"last_output"`
	VideoQuality *string `json:"video_quality"`
	OutputFile   *string `json:"output_file"`
}

type VideoTask struct {
	Task   Task   `json:"task"`
	Status Status `json:"status"`
}

var (
	Jobs     = make(map[string]*VideoTask)
	JobsLock sync.Mutex
)
