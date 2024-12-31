package webserver

type JSONAddTask struct {
	YoutubeUrl string `json:"video_url"`
	OutPath    string `json:"output_directory"`
}
