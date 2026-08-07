package ytarchive

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
)

var (
	videoTitleRe = regexp.MustCompile(`Video Title:\s*(.*?)\s*$`)
)

// StartDownload begins a ytarchive download for the given YouTube URL.
func StartDownload(ctx context.Context, url string, args []string, channelLive *common.ChannelLive, outPath string) {
	cfg := config.Get().YTArchive

	job := common.NewDownloadJob(channelLive.VideoID, *channelLive, common.StatusWaiting, outPath)
	job.Provider = "ytarchive"
	if _, ok := common.DefaultStore.Add(job); !ok {
		slog.Info("ytarchive: job already exists", "video_id", channelLive.VideoID)
		return
	}

	allArgs := append([]string(nil), args...)
	allArgs = append(allArgs, cfg.Args...)
	allArgs = append(allArgs, url)
	if cfg.Quality != "" {
		allArgs = append(allArgs, cfg.Quality)
	}

	if cfg.DelayStart != "" {
		if d, err := time.ParseDuration(cfg.DelayStart); err == nil && d > 0 {
			slog.Info("ytarchive: delaying start", "duration", d)
			select {
			case <-time.After(d):
			case <-ctx.Done():
				common.DefaultStore.SetStatus(job.VideoID, common.StatusError, "cancelled during delay")
				return
			}
		}
	}

	downloadCfg := common.DownloaderConfig{
		Executable: cfg.ExecutablePath,
		WorkingDir: cfg.WorkingDirectory,
		Args:       allArgs,
	}

	parser := &parser{job: job}
	if err := common.RunExternalDownloader(ctx, downloadCfg, job, parser); err != nil {
		slog.Error("ytarchive: download failed", "video_id", job.VideoID, "error", err)
	} else {
		slog.Info("ytarchive: download finished", "video_id", job.VideoID, "final_file", job.FinalFile)
	}
}

type parser struct {
	job *common.DownloadJob
}

func (p *parser) Parse(line string, update func(func(*common.DownloadJob))) {
	slog.Debug("ytarchive: output", "line", line)

	switch {
	case strings.Contains(line, "Video Fragments"):
		update(func(j *common.DownloadJob) {
			parts := strings.Split(line, ";")
			if len(parts) >= 3 {
				j.VideoFragments = extractValue(parts[0])
				j.AudioFragments = extractValue(parts[1])
				j.TotalSize = extractValue(parts[2])
			}
			j.Status = common.StatusDownloading
			j.Output = line
		})
	case strings.Contains(line, "Video Title"):
		update(func(j *common.DownloadJob) {
			if matches := videoTitleRe.FindStringSubmatch(line); len(matches) > 1 {
				j.ChannelLive.Title = matches[1]
			}
		})
	case strings.Contains(line, "Waiting for stream"):
		update(func(j *common.DownloadJob) {
			j.Status = common.StatusWaiting
			j.Output = line
		})
	case strings.Contains(line, "Muxing final file"):
		update(func(j *common.DownloadJob) {
			j.Status = common.StatusMuxing
		})
	case strings.Contains(line, "Livestream has been processed"):
		update(func(j *common.DownloadJob) {
			j.Status = common.StatusProcessed
		})
	case strings.Contains(line, "Final audio"):
		if filePath, ok := extractPath(line, "Final audio file: "); ok {
			update(func(j *common.DownloadJob) {
				moveDownload(filePath, j)
			})
		}
	case strings.Contains(line, "Final file"):
		if filePath, ok := extractPath(line, "Final file: "); ok {
			update(func(j *common.DownloadJob) {
				j.Status = common.StatusFinished
				j.Output = line
				moveDownload(filePath, j)
			})
		}
		discord.SendNotification(context.Background(), p.job.ChannelLive.ChannelName, p.job.ChannelLive.Title,
			fmt.Sprintf("https://www.youtube.com/watch?v=%s", p.job.VideoID), p.job.ChannelLive.ThumbnailURL, "Done")
	case strings.Contains(line, "Error retrieving player response"),
		strings.Contains(line, "unable to retrieve"),
		strings.Contains(line, "error writing the muxcmd file"),
		strings.Contains(line, "Something must have gone wrong with ffmpeg"),
		strings.Contains(line, "At least one error occurred"),
		strings.Contains(line, "ERROR: "):
		update(func(j *common.DownloadJob) {
			j.Status = common.StatusError
			j.Output = line
		})
		discord.SendNotification(context.Background(), p.job.ChannelLive.ChannelName, line,
			fmt.Sprintf("https://www.youtube.com/watch?v=%s", p.job.VideoID), p.job.ChannelLive.ThumbnailURL, "Error")
	}
}

func extractValue(part string) string {
	idx := strings.Index(part, ":")
	if idx == -1 {
		return strings.TrimSpace(part)
	}
	return strings.TrimSpace(part[idx+1:])
}

func extractPath(line, prefix string) (string, bool) {
	idx := strings.Index(line, prefix)
	if idx == -1 {
		return "", false
	}
	return strings.TrimSpace(line[idx+len(prefix):]), true
}

func moveDownload(filePath string, job *common.DownloadJob) {
	filename := filepath.Base(filePath)
	dest := filepath.Join(job.OutPath, filename)
	if err := common.MoveFile(filePath, dest); err != nil {
		slog.Error("ytarchive: failed to move file", "error", err)
		return
	}
	job.FinalFile = dest
}
