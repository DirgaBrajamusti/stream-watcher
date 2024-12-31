package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/ytarchive"
	"streamwatcher/provider/youtube"

	"github.com/kataras/golog"
)

func StartServer() {
	http.HandleFunc("/api/jobs", getDownloadJobs)
	http.HandleFunc("/api/addtask", addTask)

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

	parsedUrl, _ := url.Parse(task.YoutubeUrl)
	channelLive, err := youtube.GetVideoDetailsFromID(*youtube.ParseVideoID(parsedUrl))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	go func() {
		golog.Info("Added task for video from api: ", channelLive.VideoID)
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
	jobsSlice := make([]*common.DownloadJob, 0, len(common.DownloadJobs))
	for _, job := range common.DownloadJobs {
		jobsSlice = append(jobsSlice, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobsSlice)
}
