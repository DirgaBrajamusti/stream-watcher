package streamlink

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"strings"
	"sync"

	"github.com/kataras/golog"
)

const (
	moduleName string = "[streamlink] "
)

var waitOutputPath bool

func StartDownload(url string, args []string, channelLive *common.ChannelLive, outPath string) {
	var cmd *exec.Cmd
	var allArgs []string
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, url)
	allArgs = append(allArgs, config.AppConfig.Streamlink.Args...)

	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/C", config.AppConfig.Streamlink.ExecutablePath}, allArgs...)
		cmd = exec.Command("cmd", cmdArgs...)
		cmd.Dir = config.AppConfig.Streamlink.WorkingDirectory
		golog.Debug(moduleName, "spawning jobs in windows: ", strings.Join(allArgs, " "))
	} else {
		cmd = exec.Command(config.AppConfig.Streamlink.ExecutablePath, allArgs...)
		cmd.Dir = config.AppConfig.Streamlink.WorkingDirectory
		golog.Debug(moduleName, "spawning jobs: ", strings.Join(allArgs, " "))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		golog.Debug(moduleName, "Error creating StdoutPipe:", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		golog.Debug(moduleName, "Error creating StderrPipe:", err)
		return
	}

	common.AddDownloadJob(channelLive.VideoID, *channelLive, "Idle", "", outPath)

	// Start the command
	if err := cmd.Start(); err != nil {
		golog.Warn(moduleName, "Failed to start command: ", err)
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
		golog.Warn(moduleName, "Error waiting for command to finish: ", err)
	}

	filename := filepath.Base(common.DownloadJobs[channelLive.VideoID].FinalFile)
	if err := common.MoveFile(common.DownloadJobs[channelLive.VideoID].FinalFile, common.DownloadJobs[channelLive.VideoID].OutPath+"/"+filename); err != nil {
		golog.Warn(moduleName, "Failed to move file: ", err)
	}
	common.DownloadJobs[channelLive.VideoID].FinalFile = common.DownloadJobs[channelLive.VideoID].OutPath + "/" + filename
	discord.SendNotificationWebhook(common.DownloadJobs[channelLive.VideoID].ChannelLive.ChannelName, common.DownloadJobs[channelLive.VideoID].ChannelLive.Title, "https://twitch.tv"+common.DownloadJobs[channelLive.VideoID].ChannelLive.ChannelName, common.DownloadJobs[channelLive.VideoID].ChannelLive.ThumbnailUrl, "Done")

	golog.Debug(moduleName, "Download finished")
}

func parseOutput(output string, videoId string) {
	golog.Debug(moduleName, "Parsing output: ", output)
	common.DownloadJobsLock.Lock()
	defer common.DownloadJobsLock.Unlock()
	common.DownloadJobs[videoId].Output = output

	if waitOutputPath {
		common.DownloadJobs[videoId].FinalFile = output
		golog.Info(moduleName, "output path: ", output)
		common.DownloadJobs[videoId].Status = "Downloading"
		golog.Info(moduleName, "Downloading: ", output)
		waitOutputPath = false
	}
	if strings.Contains(output, "Writing output to") && !waitOutputPath {
		waitOutputPath = true
	}

	if strings.Contains(output, "Closing currently open stream...") {
		common.DownloadJobs[videoId].Status = "Finished"
		common.DownloadJobs[videoId].Output = output
	}
}
