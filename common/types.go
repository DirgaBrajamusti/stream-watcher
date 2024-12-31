package common

import "sync"

type ChannelLive struct {
	Title        string
	ThumbnailUrl string
	ChannelID    string
	VideoID      string
}

type DownloadJob struct {
	VideoID     string
	ChannelLive ChannelLive
	Status      string
	Output      string
	OutPath     string
}

var (
	DownloadJobs     = make(map[string]*DownloadJob)
	DownloadJobsLock sync.Mutex
)

func IsVideoIDInDownloadJobs(videoID string) bool {
	DownloadJobsLock.Lock()
	defer DownloadJobsLock.Unlock()

	_, exists := DownloadJobs[videoID]
	return exists
}

func IsChannelIDInDownloadJobs(channelID string) bool {
	DownloadJobsLock.Lock()
	defer DownloadJobsLock.Unlock()

	for _, job := range DownloadJobs {
		if job.ChannelLive.ChannelID == channelID {
			return true
		}
	}
	return false
}

func AddDownloadJob(videoID string, channelLive ChannelLive, status string, output string, outPath string) {
	DownloadJobsLock.Lock()
	defer DownloadJobsLock.Unlock()

	DownloadJobs[videoID] = &DownloadJob{
		VideoID:     videoID,
		ChannelLive: channelLive,
		Status:      status,
		Output:      output,
		OutPath:     outPath,
	}
}
