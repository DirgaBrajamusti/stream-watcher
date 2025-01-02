package webserver

type JSONAddTask struct {
	YoutubeUrl string `json:"video_url"`
	OutPath    string `json:"output_directory"`
}

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
	Version        string `json:"version"`
	State          string `json:"state"`
	LastOutput     string `json:"last_output"`
	LastUpdate     string `json:"last_update"`
	VideoFragments any    `json:"video_fragments"`
	AudioFragments any    `json:"audio_fragments"`
	TotalSize      any    `json:"total_size"`
	VideoQuality   any    `json:"video_quality"`
	OutputFile     any    `json:"output_file"`
}

type Response struct {
	Task   Task   `json:"task"`
	Status Status `json:"status"`
}

// Get Config
type Channel struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Filters          []string `json:"filters"`
	MatchDescription bool     `json:"match_description"`
	Outpath          string   `json:"outpath"`
	PictureURL       string   `json:"picture_url"`
}

type ConfigResponse struct {
	Channel []Channel `json:"channel"`
}
