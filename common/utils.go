package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/kataras/golog"
)

func CheckVideoRegex(videoTitle string, filters []string) bool {
	for _, filter := range filters {
		if regexp.MustCompile(filter).MatchString(videoTitle) {
			return true
		}
	}
	return false
}

func ReadStdout(stdout io.Reader, outputBuffer []byte, parseOutput func(string, string), videoID string, wg *sync.WaitGroup, module string) {
	defer wg.Done()
	for {
		n, err := stdout.Read(outputBuffer)
		if err != nil {
			if err != io.EOF {
				golog.Debug(fmt.Sprintf("[%s] Error reading stdout:", module), err)
			}
			return
		}

		output := string(outputBuffer[:n])
		lines := strings.Split(output, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			parseOutput(line, videoID)
		}
	}
}

func ReadStderr(stderr io.Reader, outputBuffer []byte, parseOutput func(string, string), videoID string, wg *sync.WaitGroup, module string) {
	defer wg.Done()
	for {
		n, err := stderr.Read(outputBuffer)
		if err != nil {
			if err != io.EOF {
				golog.Debug(fmt.Sprintf("[%s] Error reading stderr:", module), err)
			}
			return
		}

		output := string(outputBuffer[:n])
		lines := strings.Split(output, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			parseOutput(line, videoID)
		}
	}
}

func MoveFile(sourcePath, destPath string) error {
	// Create the destination directory if it does not exist
	destDir := filepath.Dir(destPath)
	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("[system] failed to create destination directory: %w", err)
	}

	// Rename the source file to the destination path
	golog.Debug("[system] Renaming file from ", sourcePath, " to ", destPath)
	err = os.Rename(sourcePath, destPath)
	if err != nil {
		return fmt.Errorf("[system] failed to rename file: %w", err)
	}

	return nil
}
