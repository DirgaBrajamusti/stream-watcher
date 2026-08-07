package webserver

// AddTaskRequest is the request body for adding a manual task.
type AddTaskRequest struct {
	VideoURL string `json:"video_url"`
	OutPath  string `json:"output_directory"`
}

// TaskResponse describes the task metadata returned by the API.
type TaskResponse struct {
	Title           string `json:"title"`
	VideoID         string `json:"video_id"`
	VideoURL        string `json:"video_url"`
	VideoPicture    string `json:"video_picture"`
	ChannelName     string `json:"channel_name"`
	ChannelID       string `json:"channel_id"`
	ChannelURL      string `json:"channel_url"`
	ChannelPicture  string `json:"channel_picture"`
	OutputDirectory string `json:"output_directory"`
	Platform        string `json:"platform"`
}

// StatusResponse describes the current download status.
type StatusResponse struct {
	Version        string `json:"version"`
	State          string `json:"state"`
	Provider       string `json:"provider"`
	LastOutput     string `json:"last_output"`
	LastUpdate     string `json:"last_update"`
	VideoFragments any    `json:"video_fragments"`
	AudioFragments any    `json:"audio_fragments"`
	TotalSize      any    `json:"total_size"`
	VideoQuality   any    `json:"video_quality"`
	OutputFile     any    `json:"output_file"`
}

// JobResponse is the full API response for a download job.
type JobResponse struct {
	Task   TaskResponse   `json:"task"`
	Status StatusResponse `json:"status"`
}

// ChannelResponse describes a watched channel.
type ChannelResponse struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Filters          []string `json:"filters"`
	MatchDescription bool     `json:"match_description"`
	Outpath          string   `json:"outpath"`
	PictureURL       string   `json:"picture_url"`
}

// ConfigResponse is the response for the /api/config endpoint.
type ConfigResponse struct {
	Channel []ChannelResponse `json:"channel"`
}

// MessageResponse is a simple JSON message wrapper.
type MessageResponse struct {
	Message string `json:"message"`
	Status  any    `json:"status"`
}
