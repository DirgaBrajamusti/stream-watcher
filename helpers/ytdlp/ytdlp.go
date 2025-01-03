package ytdlp

import (
	"os/exec"
	"path"
	"runtime"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"strings"
	"sync"

	"github.com/kataras/golog"
)

func StartDownload(url string, args []string, channelLive *common.ChannelLive, outPath string) {
	var cmd *exec.Cmd
	var allArgs []string
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, config.AppConfig.YT_DLP.Args...)
	allArgs = append(allArgs, "--exec", "echo Final File: {}")
	allArgs = append(allArgs, url)

	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/C", "yt-dlp"}, allArgs...)
		cmd = exec.Command("cmd", cmdArgs...)
		cmd.Dir = config.AppConfig.YT_DLP.WorkingDirectory
	} else {
		cmd = exec.Command("yt-dlp", allArgs...)
		cmd.Dir = config.AppConfig.YT_DLP.WorkingDirectory
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		golog.Debug("[yt-dlp] Error creating StdoutPipe:", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		golog.Debug("[yt-dlp] Error creating StderrPipe:", err)
		return
	}

	common.AddDownloadJob(channelLive.VideoID, *channelLive, "Idle", "", outPath)

	// Start the command
	if err := cmd.Start(); err != nil {
		golog.Warn("[yt-dlp] Failed to start command: ", err)
		return
	}
	outputBuffer := make([]byte, 4096)

	var wg sync.WaitGroup
	wg.Add(2)

	go common.ReadStdout(stdout, outputBuffer, parseOutput, channelLive.VideoID, &wg, "yt-dlp")

	// Read stderr (in case progress is written to stderr)
	go common.ReadStderr(stderr, outputBuffer, parseOutput, channelLive.VideoID, &wg, "yt-dlp")

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		golog.Warn("[yt-dlp] Error waiting for command to finish: ", err)
	}

	golog.Debug("[yt-dlp] Download finished")
}

func parseOutput(output string, videoId string) {
	common.DownloadJobsLock.Lock()
	defer common.DownloadJobsLock.Unlock()

	if strings.Contains(output, "bitrate") {
		common.DownloadJobs[videoId].Status = "Downloading"
		common.DownloadJobs[videoId].Output = output
	} else if strings.Contains(output, "fixupM3u8") {
		common.DownloadJobs[videoId].Status = "Muxing"
		common.DownloadJobs[videoId].Output = output
	} else if strings.Contains(output, "Final file:") {
		common.DownloadJobs[videoId].Status = "Finished"
		common.DownloadJobs[videoId].Output = output
		filePath := strings.Split(output, "Final file: ")[1]
		filename := path.Base(filePath)
		if err := common.MoveFile(filePath, common.DownloadJobs[videoId].OutPath+"/"+filename); err != nil {
			golog.Warn("[yt-dlp] Failed to move file: ", err)
		}
		common.DownloadJobs[videoId].FinalFile = common.DownloadJobs[videoId].OutPath + "/" + filename
		discord.SendNotificationWebhook(common.DownloadJobs[videoId].ChannelLive.ChannelName, common.DownloadJobs[videoId].ChannelLive.Title, "https://www.youtube.com/watch?v="+common.DownloadJobs[videoId].VideoID, common.DownloadJobs[videoId].ChannelLive.ThumbnailUrl, "Done")
	}
}
