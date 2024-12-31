package webserver

type JSONAddTask struct {
	YoutubeUrl string `json:"youtube_url"`
	OutPath    string `json:"out_path"`
}
