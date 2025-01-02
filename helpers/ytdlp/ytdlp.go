package ytdlp

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"streamwatcher/common"
	"streamwatcher/config"
	"streamwatcher/helpers/discord"
	"strings"

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
		fmt.Println("Error creating StdoutPipe:", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println("Error creating StderrPipe:", err)
		return
	}

	common.AddDownloadJob(channelLive.VideoID, *channelLive, "Starting", "", outPath)

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start command: %v", err)
		return
	}
	outputBuffer := make([]byte, 4096)

	// Read stdout
	go func() {
		for {
			n, err := stdout.Read(outputBuffer)
			if err != nil {
				if err != io.EOF {
					fmt.Println("Error reading stdout:", err)
				}
				return
			}

			output := string(outputBuffer[:n])
			lines := strings.Split(output, "\n")

			for _, line := range lines {
				line = strings.TrimSpace(line)
				parseOutput(line, channelLive.VideoID)
			}
		}
	}()

	// Read stderr (in case progress is written to stderr)
	go func() {
		for {
			n, err := stderr.Read(outputBuffer)
			if err != nil {
				if err != io.EOF {
					fmt.Println("Error reading stderr:", err)
				}
				return
			}

			output := string(outputBuffer[:n])
			lines := strings.Split(output, "\n")

			for _, line := range lines {
				line = strings.TrimSpace(line)
				// golog.Debug("[yt-dlp] output: " + line)
				parseOutput(line, channelLive.VideoID)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		golog.Warn("[yt-dlp] Error waiting for command to finish: ", err)
	}

	golog.Info("[yt-dlp] Download finished")
}

func parseOutput(output string, videoId string) {
	// golog.Info("Parsing output: ", output)
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
		if err := moveFile(filePath, common.DownloadJobs[videoId].OutPath+"/"+filename); err != nil {
			golog.Warn("[yt-dlp] Failed to move file: ", err)
		}
		discord.SendNotificationWebhook(common.DownloadJobs[videoId].ChannelLive.ChannelID, common.DownloadJobs[videoId].ChannelLive.Title, "https://www.youtube.com/watch?v="+common.DownloadJobs[videoId].VideoID, common.DownloadJobs[videoId].ChannelLive.ThumbnailUrl, "Done")
	}
}

func moveFile(sourcePath, destPath string) error {
	// Create the destination directory if it does not exist
	destDir := filepath.Dir(destPath)
	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("[ytdlp] failed to create destination directory: %w", err)
	}

	// Rename the source file to the destination path
	golog.Debug("[ytdlp] Renaming file from ", sourcePath, " to ", destPath)
	err = os.Rename(sourcePath, destPath)
	if err != nil {
		return fmt.Errorf("[ytdlp] failed to rename file: %w", err)
	}

	return nil
}
