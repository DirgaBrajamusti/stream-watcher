package webserver

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/ytarchive"
	"streamwatcher/provider/twitch"
	"streamwatcher/provider/youtube"
	"strings"
	"sync"
	"time"

	"github.com/kataras/golog"
)

//go:embed frontend/dist/*
var staticFiles embed.FS

// spaFileSystem is a custom file system wrapper for SPA
type spaFileSystem struct {
	http.FileSystem
	index string
}

func (fs spaFileSystem) Open(path string) (http.File, error) {
	f, err := fs.FileSystem.Open(path)
	if err != nil {
		return fs.FileSystem.Open(fs.index)
	}
	return f, err
}

func StartServer() {
	// API routes
	http.HandleFunc("/api/tasks", getDownloadJobs)
	http.HandleFunc("/api/task", addTask)
	http.HandleFunc("/api/config/toml", tomlConfig)
	http.HandleFunc("/api/config", getConfig)

	// Static files handling
	staticFS, err := fs.Sub(staticFiles, "frontend/dist")
	if err != nil {
		golog.Fatal(err)
	}

	spa := spaFileSystem{
		FileSystem: http.FS(staticFS),
		index:      "index.html",
	}

	http.Handle("/", http.FileServer(spa))
	golog.Info(http.ListenAndServe(fmt.Sprintf("%s:%s", config.AppConfig.Webserver.Host, config.AppConfig.Webserver.Port), nil))
}
func addTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var task JSONAddTask
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsedUrl, err := url.Parse(task.YoutubeUrl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if parsedUrl.Host == "twitch.tv" || parsedUrl.Host == "www.twitch.tv" {
		twitchUsername := strings.TrimPrefix(parsedUrl.Path, "/")
		channelLive, err := twitch.GetChannelInfo(twitchUsername)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if channelLive == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			response := map[string]interface{}{
				"message": "Task not added",
				"status":  "Channel maybe offline",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		go func() {
			twitch.TwitchStartDownload(twitchUsername, channelLive, task.OutPath)
		}()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		response := map[string]interface{}{
			"message": "Task added successfully",
			"status":  channelLive,
		}
		json.NewEncoder(w).Encode(response)
	} else {
		url := youtube.ParseVideoID(parsedUrl)
		if url == nil {
			http.Error(w, "Invalid YouTube URL", http.StatusBadRequest)
			return
		} else {
			channelLive, err := youtube.GetVideoDetailsFromID(*url)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			go func() {
				golog.Info("[webserver] Added task for video from api: ", channelLive.VideoID)
				ytarchive.StartDownload("https://www.youtube.com/watch?v="+channelLive.VideoID, []string{}, channelLive, task.OutPath)
			}()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			response := map[string]interface{}{
				"message": "Task added successfully",
				"status":  channelLive,
			}
			json.NewEncoder(w).Encode(response)
		}
	}

}
func getDownloadJobs(w http.ResponseWriter, r *http.Request) {
	common.DownloadJobsLock.Lock()
	defer common.DownloadJobsLock.Unlock()

	jobsSlice := convertDownloadJobsToResponse(common.DownloadJobs)
	if jobsSlice == nil {
		jobsSlice = []Response{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobsSlice)
}

func convertDownloadJobsToResponse(jobs map[string]*common.DownloadJob) []Response {
	var responses []Response
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Precompile regex patterns
	// sizePattern := regexp.MustCompile(`size=\s*([\d.]+[KMG]i?B?)\s*time=.*bitrate=([\d.]+[kMG]?bits/s)\s*speed=([\d.]+x)`)
	// fragmentsPattern := regexp.MustCompile(`Video Fragments:\s*(\d+);\s*Audio Fragments:\s*(\d+);\s*Total Downloaded:\s*([\d.]+[KMG]i?B?)`)

	for _, job := range jobs {
		wg.Add(1)
		go func(job *common.DownloadJob) {
			defer wg.Done()
			response := Response{
				Task: Task{
					Title:           job.ChannelLive.Title,
					VideoID:         job.VideoID,
					VideoPicture:    job.ChannelLive.ThumbnailUrl,
					ChannelName:     job.ChannelLive.ChannelName,
					ChannelID:       job.ChannelLive.ChannelID,
					ChannelPicture:  job.ChannelLive.ChannelPicture,
					OutputDirectory: job.OutPath,
				},
				Status: Status{
					Version:        "",
					State:          job.Status,
					LastOutput:     job.Output,
					LastUpdate:     job.ChannelLive.DateCrawled,
					VideoFragments: job.VideoFragments,
					AudioFragments: job.AudioFragments,
					TotalSize:      job.TotalSize,
					VideoQuality:   nil,
					OutputFile:     job.FinalFile,
				},
			}
			mu.Lock()
			responses = append(responses, response)
			mu.Unlock()
		}(job)
	}

	wg.Wait()

	sort.Slice(responses, func(i, j int) bool {
		timeI, _ := time.Parse(time.RFC3339, responses[i].Status.LastUpdate)
		timeJ, _ := time.Parse(time.RFC3339, responses[j].Status.LastUpdate)
		return timeI.Before(timeJ)
	})

	return responses
}

func parseOutput(output string, sizePattern, fragmentsPattern *regexp.Regexp) (map[string]string, error) {
	if strings.HasPrefix(output, "size=") {
		matches := sizePattern.FindStringSubmatch(output)
		if len(matches) != 4 {
			return nil, fmt.Errorf("failed to parse output")
		}

		result := map[string]string{
			"TotalSize":      matches[1],
			"VideoFragments": "0",
			"AudioFragments": "0",
		}
		return result, nil
	} else if strings.HasPrefix(output, "Video Fragments: ") {
		matches := fragmentsPattern.FindStringSubmatch(output)
		if len(matches) != 4 {
			return nil, fmt.Errorf("failed to parse output")
		}
		result := map[string]string{
			"TotalSize":      matches[3],
			"VideoFragments": matches[1],
			"AudioFragments": matches[2],
		}
		return result, nil
	}
	return nil, fmt.Errorf("output does not start with expected pattern")
}

func getConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var response ConfigResponse

	for _, channel := range config.AppConfig.YouTubeChannel {
		response.Channel = append(response.Channel, Channel{
			ID:               channel.ID,
			Name:             channel.Name,
			Filters:          channel.Filters,
			MatchDescription: false,
			Outpath:          channel.OutPath,
			PictureURL:       "",
		})
	}

	json.NewEncoder(w).Encode(response)
}

func tomlConfig(w http.ResponseWriter, r *http.Request) {
	filePath := "./config.toml"

	switch r.Method {
	case http.MethodGet:
		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "Unable to read config file", http.StatusInternalServerError)
			golog.Warn("Error reading config file:", err)
			return
		}
		w.Write(content)

	case http.MethodPut:
		content, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read request body", http.StatusBadRequest)
			golog.Warn("Error reading request body:", err)
			return
		}

		err = os.WriteFile(filePath, content, 0644)
		if err != nil {
			http.Error(w, "Unable to write to config file", http.StatusInternalServerError)
			golog.Warn("Error writing to config file:", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}
