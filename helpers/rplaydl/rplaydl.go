package rplaydl

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
)

// StartDownload begins a yt-dlp download for the given RPLAY playback URL.
func StartDownload(ctx context.Context, playbackURL string, channelLive *common.ChannelLive, outPath string) {
	cfg := config.Get().YTDLP

	job := common.NewDownloadJob(channelLive.VideoID, *channelLive, common.StatusIdle, outPath)
	job.Provider = "rplay"
	if _, ok := common.DefaultStore.Add(job); !ok {
		slog.Info("rplay: job already exists", "video_id", channelLive.VideoID)
		return
	}

	filename := fmt.Sprintf("[%s]_RPLAY_%s.mp4", time.Now().Format("2006-01-02_15_04_05"), channelLive.ChannelName)

	allArgs := []string{
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
		"--referer", "https://rplay.live/",
		"-o", filename,
	}
	allArgs = append(allArgs, cfg.Args...)
	allArgs = append(allArgs, "--print", `after_move:"Final File: %(filepath)s"`)
	allArgs = append(allArgs, "--no-quiet")
	allArgs = append(allArgs, playbackURL)

	downloadCfg := common.DownloaderConfig{
		Executable: cfg.ExecutablePath,
		WorkingDir: cfg.WorkingDirectory,
		Args:       allArgs,
	}

	parser := &parser{job: job}
	if err := common.RunExternalDownloader(ctx, downloadCfg, job, parser); err != nil {
		slog.Error("rplay: download failed", "video_id", job.VideoID, "error", err)
	} else {
		slog.Info("rplay: download finished", "video_id", job.VideoID, "final_file", job.FinalFile)
	}
}

type parser struct {
	job *common.DownloadJob
}

func (p *parser) Parse(line string, update func(func(*common.DownloadJob))) {
	slog.Debug("rplay: output", "line", line)

	switch {
	case strings.Contains(line, "bitrate"):
		update(func(j *common.DownloadJob) {
			j.Status = common.StatusDownloading
			j.Output = line
		})
	case strings.Contains(line, "fixupM3u8"):
		update(func(j *common.DownloadJob) {
			j.Status = common.StatusMuxing
			j.Output = line
		})
	case strings.Contains(line, "Final file:"):
		parts := strings.SplitN(line, "Final file: ", 2)
		if len(parts) == 2 {
			filePath := strings.TrimSpace(parts[1])
			update(func(j *common.DownloadJob) {
				j.Status = common.StatusFinished
				j.Output = line
				filename := filepath.Base(filePath)
				dest := filepath.Join(j.OutPath, filename)
				if err := common.MoveFile(filePath, dest); err != nil {
					slog.Error("rplay: failed to move file", "error", err)
				} else {
					j.FinalFile = dest
				}
			})
		}
		discord.SendNotification(context.Background(), p.job.ChannelLive.ChannelName, p.job.ChannelLive.Title,
			fmt.Sprintf("https://rplay.live/creator/%s", p.job.ChannelLive.ChannelID), p.job.ChannelLive.ThumbnailURL, "Done")
	}
}
