package webserver

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/ytarchive"
	"streamwatcher/provider/twitch"
	"streamwatcher/provider/youtube"
)

const appVersion = "0.1.0"

//go:embed frontend/dist/*
var staticFiles embed.FS

// spaFileSystem is a custom file system wrapper for a single-page application.
type spaFileSystem struct {
	fs    http.FileSystem
	index string
}

func (s spaFileSystem) Open(path string) (http.File, error) {
	f, err := s.fs.Open(path)
	if err != nil {
		return s.fs.Open(s.index)
	}
	return f, nil
}

// Server wraps the HTTP server used by the application.
type Server struct {
	srv *http.Server
}

// NewServer creates a web server configured with the current application settings.
func NewServer() *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/tasks", getDownloadJobs)
	mux.HandleFunc("POST /api/task", addTask)
	mux.HandleFunc("GET /api/config", getConfig)
	mux.HandleFunc("GET /api/config/toml", tomlConfig)
	mux.HandleFunc("PUT /api/config/toml", tomlConfig)

	staticFS, err := fs.Sub(staticFiles, "frontend/dist")
	if err != nil {
		slog.Warn("webserver: frontend dist not found; static files will not be served", "error", err)
	} else {
		spa := spaFileSystem{fs: http.FS(staticFS), index: "index.html"}
		mux.Handle("/", http.FileServer(spa))
	}

	return &Server{
		srv: &http.Server{
			Addr:         config.ListenAddr(),
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start starts the web server in a goroutine.
func (s *Server) Start() {
	go func() {
		slog.Info("webserver: listening", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("webserver: server error", "error", err)
		}
	}()
}

// Stop gracefully shuts down the web server.
func (s *Server) Stop(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func getDownloadJobs(w http.ResponseWriter, r *http.Request) {
	jobs := common.DefaultStore.List()
	responses := make([]JobResponse, 0, len(jobs))
	for _, job := range jobs {
		responses = append(responses, jobToResponse(job))
	}

	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Status.LastUpdate < responses[j].Status.LastUpdate
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		slog.Error("webserver: failed to encode jobs", "error", err)
	}
}

func addTask(w http.ResponseWriter, r *http.Request) {
	var req AddTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.VideoURL == "" {
		http.Error(w, "video_url is required", http.StatusBadRequest)
		return
	}

	parsed, err := url.Parse(req.VideoURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid url: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if isTwitchURL(parsed.Host) {
		username := strings.TrimPrefix(parsed.Path, "/")
		live, err := twitch.GetChannelInfo(ctx, username)
		if err != nil {
			http.Error(w, fmt.Sprintf("twitch lookup failed: %v", err), http.StatusBadRequest)
			return
		}
		if live == nil {
			writeJSON(w, http.StatusCreated, MessageResponse{Message: "Task not added", Status: "channel is offline"})
			return
		}
		go twitch.StartDownload(ctx, username, live, req.OutPath)
		writeJSON(w, http.StatusCreated, MessageResponse{Message: "Task added successfully", Status: live})
		return
	}

	videoID, ok := youtube.ParseVideoID(parsed)
	if !ok {
		http.Error(w, "invalid YouTube URL", http.StatusBadRequest)
		return
	}

	live, err := youtube.GetVideoDetailsFromID(ctx, videoID)
	if err != nil {
		http.Error(w, fmt.Sprintf("youtube lookup failed: %v", err), http.StatusBadRequest)
		return
	}
	go ytarchive.StartDownload(ctx,
		fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
		[]string{}, live, req.OutPath)
	writeJSON(w, http.StatusCreated, MessageResponse{Message: "Task added successfully", Status: live})
}

func getConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	resp := ConfigResponse{Channel: make([]ChannelResponse, 0, len(cfg.YouTubeChannel))}
	for _, channel := range cfg.YouTubeChannel {
		resp.Channel = append(resp.Channel, ChannelResponse{
			ID:      channel.ID,
			Name:    channel.Name,
			Filters: channel.Filters,
			Outpath: channel.OutPath,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func tomlConfig(w http.ResponseWriter, r *http.Request) {
	const filePath = "./config.toml"

	switch r.Method {
	case http.MethodGet:
		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "Unable to read config file", http.StatusInternalServerError)
			slog.Warn("webserver: failed to read config file", "error", err)
			return
		}
		w.Header().Set("Content-Type", "application/toml")
		w.Write(content)

	case http.MethodPut:
		content, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read request body", http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(filePath, content, 0o644); err != nil {
			http.Error(w, "Unable to write config file", http.StatusInternalServerError)
			slog.Warn("webserver: failed to write config file", "error", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func isTwitchURL(host string) bool {
	return host == "twitch.tv" || host == "www.twitch.tv"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("webserver: failed to encode response", "error", err)
	}
}

func jobToResponse(job *common.DownloadJob) JobResponse {
	videoURL, channelURL := platformURLs(job)
	return JobResponse{
		Task: TaskResponse{
			Title:           job.ChannelLive.Title,
			VideoID:         job.VideoID,
			VideoURL:        videoURL,
			VideoPicture:    job.ChannelLive.ThumbnailURL,
			ChannelName:     job.ChannelLive.ChannelName,
			ChannelID:       job.ChannelLive.ChannelID,
			ChannelURL:      channelURL,
			ChannelPicture:  job.ChannelLive.ChannelPicture,
			OutputDirectory: job.OutPath,
			Platform:        job.ChannelLive.Platform,
		},
		Status: StatusResponse{
			Version:        appVersion,
			State:          string(job.Status),
			Provider:       job.Provider,
			LastOutput:     job.Output,
			LastUpdate:     job.UpdatedAt.Format(time.RFC3339),
			VideoFragments: job.VideoFragments,
			AudioFragments: job.AudioFragments,
			TotalSize:      job.TotalSize,
			OutputFile:     job.FinalFile,
		},
	}
}

func platformURLs(job *common.DownloadJob) (videoURL, channelURL string) {
	switch job.ChannelLive.Platform {
	case "twitch":
		channelURL = fmt.Sprintf("https://twitch.tv/%s", job.ChannelLive.ChannelName)
		videoURL = channelURL
	case "youtube":
		channelURL = fmt.Sprintf("https://www.youtube.com/channel/%s", job.ChannelLive.ChannelID)
		videoURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", job.VideoID)
	case "rplay":
		channelURL = fmt.Sprintf("https://rplay.live/creator/%s", job.ChannelLive.ChannelID)
		videoURL = channelURL
	default:
		videoURL = ""
		channelURL = ""
	}
	return videoURL, channelURL
}
