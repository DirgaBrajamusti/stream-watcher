package ytdlp

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

// StartDownload begins a yt-dlp download for the given URL.
func StartDownload(ctx context.Context, url string, args []string, channelLive *common.ChannelLive, outPath string) {
	cfg := config.Get().YTDLP

	job := common.NewDownloadJob(channelLive.VideoID, *channelLive, common.StatusIdle, outPath)
	job.Provider = "yt-dlp"
	if _, ok := common.DefaultStore.Add(job); !ok {
		slog.Info("yt-dlp: job already exists", "video_id", channelLive.VideoID)
		return
	}

	allArgs := append([]string(nil), args...)
	allArgs = append(allArgs, cfg.Args...)
	allArgs = append(allArgs, "--print", `after_move:"Final File: %(filepath)s"`)
	allArgs = append(allArgs, "--no-quiet")
	allArgs = append(allArgs, url)

	downloadCfg := common.DownloaderConfig{
		Executable: cfg.ExecutablePath,
		WorkingDir: cfg.WorkingDirectory,
		Args:       allArgs,
	}

	parser := &parser{job: job}
	if err := common.RunExternalDownloader(ctx, downloadCfg, job, parser); err != nil {
		slog.Error("yt-dlp: download failed", "video_id", job.VideoID, "error", err)
	} else {
		slog.Info("yt-dlp: download finished", "video_id", job.VideoID, "final_file", job.FinalFile)
	}
}

type parser struct {
	job *common.DownloadJob
}

func (p *parser) Parse(line string, update func(func(*common.DownloadJob))) {
	slog.Debug("yt-dlp: output", "line", line)

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
					slog.Error("yt-dlp: failed to move file", "error", err)
				} else {
					j.FinalFile = dest
				}
			})
		}
		discord.SendNotification(context.Background(), p.job.ChannelLive.ChannelName, p.job.ChannelLive.Title,
			fmt.Sprintf("https://www.youtube.com/watch?v=%s", p.job.VideoID), p.job.ChannelLive.ThumbnailURL, "Done")
	}
}
