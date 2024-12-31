package ytarchive

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
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
	allArgs = append(allArgs, config.AppConfig.YTArchive.Args...)
	allArgs = append(allArgs, "--temporary-dir", config.AppConfig.YTArchive.WorkingDirectory)
	allArgs = append(allArgs, "--start-delay", config.AppConfig.YTArchive.DelayStart)
	allArgs = append(allArgs, url)
	allArgs = append(allArgs, config.AppConfig.YTArchive.Quality)

	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/C", "ytarchive"}, allArgs...)
		cmd = exec.Command("cmd", cmdArgs...)
	} else {
		cmd = exec.Command("ytarchive", allArgs...)
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
				golog.Debug("ytarchive: " + line)
				parseOutput(line, channelLive.VideoID)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		fmt.Println("Error waiting for command to finish:", err)
	}

	fmt.Println("Download finished")
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
			golog.Warn("Failed to move file: ", err)
		}
		// os.Rename(filePath, common.DownloadJobs[videoId].OutPath+"/"+filename)
		discord.SendNotificationWebhook(common.DownloadJobs[videoId].ChannelLive.ChannelID, common.DownloadJobs[videoId].ChannelLive.Title, "https://www.youtube.com/watch?v="+common.DownloadJobs[videoId].VideoID, common.DownloadJobs[videoId].ChannelLive.ThumbnailUrl, "Done")
	} else if strings.Contains(output, "Error retrieving player response") || strings.Contains(output, "unable to retrieve") || strings.Contains(output, "error writing the muxcmd file") || strings.Contains(output, "Something must have gone wrong with ffmpeg") || strings.Contains(output, "At least one error occurred") || strings.Contains(output, "ERROR: ") {
		common.DownloadJobs[videoId].Status = "Error"
		common.DownloadJobs[videoId].Output = output
		discord.SendNotificationWebhook(common.DownloadJobs[videoId].ChannelLive.ChannelID, common.DownloadJobs[videoId].ChannelLive.Title, "https://www.youtube.com/watch?v="+common.DownloadJobs[videoId].VideoID, common.DownloadJobs[videoId].ChannelLive.ThumbnailUrl, "Error")
	}
}

func moveFile(sourcePath, destPath string) error {
	// Open the source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	golog.Debug("Creating destination file: ", destPath)
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy the contents from the source file to the destination file
	golog.Debug("Copying file contents...")
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Remove the source file
	err = os.Remove(sourcePath)
	golog.Debug("Removing source file: ", sourcePath)
	if err != nil {
		return fmt.Errorf("failed to remove source file: %w", err)
	}

	return nil
}
