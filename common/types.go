package common

import (
	"regexp"
	"sync"
	"time"
)

// DownloadStatus describes the state of a download job.
type DownloadStatus string

const (
	StatusIdle        DownloadStatus = "Idle"
	StatusWaiting     DownloadStatus = "Waiting"
	StatusRecording   DownloadStatus = "Recording"
	StatusDownloading DownloadStatus = "Downloading"
	StatusMuxing      DownloadStatus = "Muxing"
	StatusProcessed   DownloadStatus = "Processed"
	StatusFinished    DownloadStatus = "Finished"
	StatusError       DownloadStatus = "Error"
)

// ChannelLive holds metadata about a currently live stream.
type ChannelLive struct {
	Title          string
	Game           string
	ThumbnailURL   string
	ChannelID      string
	VideoID        string
	ChannelName    string
	ChannelPicture string
	DateCrawled    time.Time
	MembersOnly    bool
	Platform       string // "youtube" or "twitch"
}

// DownloadJob represents an active or completed stream recording.
type DownloadJob struct {
	VideoID        string
	ChannelLive    ChannelLive
	Status         DownloadStatus
	Output         string
	AudioFragments string
	VideoFragments string
	TotalSize      string
	OutPath        string
	FinalFile      string
	Provider       string // e.g. "ytarchive", "yt-dlp", "streamlink"
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewDownloadJob creates a new job with the current timestamp.
func NewDownloadJob(videoID string, live ChannelLive, status DownloadStatus, outPath string) *DownloadJob {
	now := time.Now().UTC()
	return &DownloadJob{
		VideoID:     videoID,
		ChannelLive: live,
		Status:      status,
		OutPath:     outPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// JobStore is a thread-safe store of active download jobs.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*DownloadJob
}

// NewJobStore creates a new JobStore.
func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*DownloadJob)}
}

// Get returns a job by video ID.
func (s *JobStore) Get(videoID string) (*DownloadJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[videoID]
	return j, ok
}

// Add registers a job. If a job with the same video ID already exists, the existing
// job is returned and ok is false.
func (s *JobStore) Add(job *DownloadJob) (*DownloadJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.jobs[job.VideoID]; ok {
		return existing, false
	}
	s.jobs[job.VideoID] = job
	return job, true
}

// Update applies a mutation to a job. If the job does not exist, ok is false.
func (s *JobStore) Update(videoID string, fn func(*DownloadJob)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[videoID]
	if !ok {
		return false
	}
	fn(j)
	j.UpdatedAt = time.Now().UTC()
	return true
}

// SetStatus is a convenience update for the job status.
func (s *JobStore) SetStatus(videoID string, status DownloadStatus, output string) bool {
	return s.Update(videoID, func(j *DownloadJob) {
		j.Status = status
		if output != "" {
			j.Output = output
		}
	})
}

// Exists reports whether a non-errored job exists for the video ID.
func (s *JobStore) Exists(videoID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[videoID]
	if !ok {
		return false
	}
	return j.Status != StatusError
}

// IsChannelBusy reports whether a channel has an active or completed-but-not-errored
// download job.
func (s *JobStore) IsChannelBusy(channelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.jobs {
		if j.ChannelLive.ChannelID == channelID && j.Status != StatusError && j.Status != StatusFinished {
			return true
		}
	}
	return false
}

// List returns a snapshot of all jobs.
func (s *JobStore) List() []*DownloadJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*DownloadJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

// Remove deletes a job from the store.
func (s *JobStore) Remove(videoID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, videoID)
}

// MatchFilters returns true if the title matches any of the provided regex filters.
// An empty filter string acts as a wildcard and matches everything. An empty filter
// list (or nil) also acts as a wildcard.
func MatchFilters(title string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f == "" {
			return true
		}
		if re, ok := compileRegex(f); ok && re.MatchString(title) {
			return true
		}
	}
	return false
}

// regexCache avoids recompiling regex patterns.
var (
	regexCacheMu sync.RWMutex
	regexCache   = make(map[string]*regexpPair)
)

// DefaultStore is the global job store used by the application.
var DefaultStore = NewJobStore()

type regexpPair struct {
	re *regexp.Regexp
	ok bool
}

func compileRegex(pattern string) (*regexp.Regexp, bool) {
	regexCacheMu.RLock()
	cached, ok := regexCache[pattern]
	regexCacheMu.RUnlock()
	if ok {
		return cached.re, cached.ok
	}

	re, err := regexp.Compile(pattern)
	regexCacheMu.Lock()
	regexCache[pattern] = &regexpPair{re: re, ok: err == nil}
	regexCacheMu.Unlock()
	return re, err == nil
}
