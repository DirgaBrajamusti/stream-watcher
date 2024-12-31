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
	Version        string  `json:"version"`
	State          string  `json:"state"`
	LastOutput     string  `json:"last_output"`
	LastUpdate     *string `json:"last_update"`
	VideoFragments int     `json:"video_fragments"`
	AudioFragments int     `json:"audio_fragments"`
	TotalSize      string  `json:"total_size"`
	VideoQuality   string  `json:"video_quality"`
	OutputFile     string  `json:"output_file"`
}

type Response struct {
	Task   Task   `json:"task"`
	Status Status `json:"status"`
}
