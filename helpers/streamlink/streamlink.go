package streamlink

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
)

type parser struct {
	job            *common.DownloadJob
	waitOutputPath bool
}

// StartDownload begins a streamlink download for the given URL.
func StartDownload(ctx context.Context, url string, args []string, channelLive *common.ChannelLive, outPath string) {
	cfg := config.Get().Streamlink

	job := common.NewDownloadJob(channelLive.VideoID, *channelLive, common.StatusIdle, outPath)
	job.Provider = "streamlink"
	if _, ok := common.DefaultStore.Add(job); !ok {
		slog.Info("streamlink: job already exists", "video_id", channelLive.VideoID)
		return
	}

	allArgs := append([]string(nil), args...)
	allArgs = append(allArgs, url)
	allArgs = append(allArgs, cfg.Args...)

	downloadCfg := common.DownloaderConfig{
		Executable: cfg.ExecutablePath,
		WorkingDir: cfg.WorkingDirectory,
		Args:       allArgs,
	}

	p := &parser{job: job}
	if err := common.RunExternalDownloader(ctx, downloadCfg, job, p); err != nil {
		slog.Error("streamlink: download failed", "video_id", job.VideoID, "error", err)
	} else {
		slog.Info("streamlink: download finished", "video_id", job.VideoID, "final_file", job.FinalFile)
	}
}

func (p *parser) Parse(line string, update func(func(*common.DownloadJob))) {
	slog.Debug("streamlink: output", "line", line)

	switch {
	case p.waitOutputPath:
		update(func(j *common.DownloadJob) {
			j.FinalFile = strings.TrimSpace(line)
			j.Status = common.StatusDownloading
		})
		p.waitOutputPath = false
	case strings.Contains(line, "Writing output to"):
		p.waitOutputPath = true
	case strings.Contains(line, "Closing currently open stream..."):
		update(func(j *common.DownloadJob) {
			j.Status = common.StatusFinished
			j.Output = line
			moveDownload(j)
		})
		discord.SendNotification(context.Background(), p.job.ChannelLive.ChannelName, p.job.ChannelLive.Title,
			fmt.Sprintf("https://twitch.tv/%s", p.job.ChannelLive.ChannelName), p.job.ChannelLive.ThumbnailURL, "Done")
	}
}

func moveDownload(job *common.DownloadJob) {
	if job.FinalFile == "" {
		return
	}
	filename := filepath.Base(job.FinalFile)
	dest := filepath.Join(job.OutPath, filename)
	if err := common.MoveFile(job.FinalFile, dest); err != nil {
		slog.Error("streamlink: failed to move file", "error", err)
		return
	}
	job.FinalFile = dest
}
