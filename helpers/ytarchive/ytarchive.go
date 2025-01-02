package ytarchive

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
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
	allArgs = append(allArgs, config.AppConfig.YTArchive.Args...)
	// allArgs = append(allArgs, "--temporary-dir", config.AppConfig.YTArchive.WorkingDirectory)
	allArgs = append(allArgs, "--start-delay", config.AppConfig.YTArchive.DelayStart)
	allArgs = append(allArgs, url)
	allArgs = append(allArgs, config.AppConfig.YTArchive.Quality)

	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/C", "ytarchive"}, allArgs...)
		cmd = exec.Command("cmd", cmdArgs...)
		cmd.Dir = config.AppConfig.YTArchive.WorkingDirectory
	} else {
		cmd = exec.Command("ytarchive", allArgs...)
		cmd.Dir = config.AppConfig.YTArchive.WorkingDirectory
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		golog.Debug("[ytarchive] Error creating StdoutPipe:", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		golog.Debug("[ytarchive] Error creating StderrPipe:", err)
		return
	}

	common.AddDownloadJob(channelLive.VideoID, *channelLive, "Idle", "", outPath)

	// Start the command
	if err := cmd.Start(); err != nil {
		golog.Warn("[ytarchive] Failed to start command: ", err)
		return
	}
	outputBuffer := make([]byte, 4096)

	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		for {
			n, err := stdout.Read(outputBuffer)
			if err != nil {
				if err != io.EOF {
					golog.Debug("[ytarchive] Error reading stdout:", err)
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
		defer wg.Done()
		for {
			n, err := stderr.Read(outputBuffer)
			if err != nil {
				if err != io.EOF {
					golog.Debug("[ytarchive] Error reading stderr:", err)
				}
				return
			}

			output := string(outputBuffer[:n])
			lines := strings.Split(output, "\n")

			for _, line := range lines {
				line = strings.TrimSpace(line)
				golog.Debug("[ytarchive] output: " + line)
				parseOutput(line, channelLive.VideoID)
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		golog.Warn("[ytarchive] Error waiting for command to finish:", err)
	}

	golog.Debug("[ytarchive] Exited")
}

func parseOutput(output string, videoId string) {
	common.DownloadJobsLock.Lock()
	defer common.DownloadJobsLock.Unlock()
	if strings.Contains(output, "Video Title") {
		re := regexp.MustCompile(`Video Title:\s*(.*?)\s*$`)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			common.DownloadJobs[videoId].ChannelLive.Title = matches[1]
		}
	} else if strings.Contains(output, "Video Fragments") {
		common.DownloadJobs[videoId].Status = "Downloading"
		common.DownloadJobs[videoId].Output = output
	} else if strings.Contains(output, "Waiting for stream") {
		common.DownloadJobs[videoId].Output = output
		common.DownloadJobs[videoId].Status = "Waiting"
	} else if strings.Contains(output, "Muxing final file") {
		common.DownloadJobs[videoId].Status = "Muxing"
	} else if strings.Contains(output, "Livestream has been processed") {
		common.DownloadJobs[videoId].Status = "Processed"
	} else if strings.Contains(output, "Final file") {
		common.DownloadJobs[videoId].Status = "Finished"
		common.DownloadJobs[videoId].Output = output
		filePath := strings.Split(output, "Final file: ")[1]
		filename := path.Base(filePath)
		if err := moveFile(filePath, common.DownloadJobs[videoId].OutPath+"/"+filename); err != nil {
			golog.Warn("[ytarchive] Failed to move file: ", err)
		}
		common.DownloadJobs[videoId].FinalFile = common.DownloadJobs[videoId].OutPath + "/" + filename
		discord.SendNotificationWebhook(common.DownloadJobs[videoId].ChannelLive.ChannelName, common.DownloadJobs[videoId].ChannelLive.Title, "https://www.youtube.com/watch?v="+common.DownloadJobs[videoId].VideoID, common.DownloadJobs[videoId].ChannelLive.ThumbnailUrl, "Done")
	} else if strings.Contains(output, "Error retrieving player response") || strings.Contains(output, "unable to retrieve") || strings.Contains(output, "error writing the muxcmd file") || strings.Contains(output, "Something must have gone wrong with ffmpeg") || strings.Contains(output, "At least one error occurred") || strings.Contains(output, "ERROR: ") {
		common.DownloadJobs[videoId].Status = "Error"
		common.DownloadJobs[videoId].Output = output
		discord.SendNotificationWebhook(common.DownloadJobs[videoId].ChannelLive.ChannelName, common.DownloadJobs[videoId].ChannelLive.Title, "https://www.youtube.com/watch?v="+common.DownloadJobs[videoId].VideoID, common.DownloadJobs[videoId].ChannelLive.ThumbnailUrl, "Error")
	}
}

func moveFile(sourcePath, destPath string) error {
	// Create the destination directory if it does not exist
	destDir := filepath.Dir(destPath)
	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("[ytarchive] failed to create destination directory: %w", err)
	}

	// Rename the source file to the destination path
	golog.Debug("[ytarchive] Renaming file from ", sourcePath, " to ", destPath)
	err = os.Rename(sourcePath, destPath)
	if err != nil {
		return fmt.Errorf("[ytarchive] failed to rename file: %w", err)
	}

	return nil
}
