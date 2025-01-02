package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/ytarchive"
	"streamwatcher/provider/youtube"
	"strings"
	"sync"
	"time"

	"github.com/kataras/golog"
)

func StartServer() {
	http.HandleFunc("/api/tasks", getDownloadJobs)
	http.HandleFunc("/api/task", addTask)

	http.Handle("/", http.FileServer(http.Dir("./helpers/webserver/frontend/dist")))
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
	channelLive, err := youtube.GetVideoDetailsFromID(*youtube.ParseVideoID(parsedUrl))
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
func getDownloadJobs(w http.ResponseWriter, r *http.Request) {
	common.DownloadJobsLock.Lock()
	defer common.DownloadJobsLock.Unlock()

	// Convert the DownloadJobs map to a slice for JSON response
	// jobsSlice := make([]*common.DownloadJob, 0, len(common.DownloadJobs))
	// for _, job := range common.DownloadJobs {
	// 	jobsSlice = append(jobsSlice, job)
	// }
	jobsSlice := convertDownloadJobsToResponse(common.DownloadJobs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobsSlice)
}

func convertDownloadJobsToResponse(jobs map[string]*common.DownloadJob) []Response {
	var responses []Response
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Precompile regex patterns
	sizePattern := regexp.MustCompile(`size=\s*([\d.]+[KMG]i?B?)\s*time=.*bitrate=([\d.]+[kMG]?bits/s)\s*speed=([\d.]+x)`)
	fragmentsPattern := regexp.MustCompile(`Video Fragments:\s*(\d+);\s*Audio Fragments:\s*(\d+);\s*Total Downloaded:\s*([\d.]+[KMG]i?B?)`)

	for _, job := range jobs {
		wg.Add(1)
		go func(job *common.DownloadJob) {
			defer wg.Done()
			outputParsed, _ := parseOutput(job.Output, sizePattern, fragmentsPattern)
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
					VideoFragments: outputParsed["VideoFragments"],
					AudioFragments: outputParsed["AudioFragments"],
					TotalSize:      outputParsed["TotalSize"],
					VideoQuality:   "",
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
