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
	golog.Debug("[system] Moving file from ", sourcePath, " to ", destPath)
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("[system] failed to create destination directory: %w", err)
	}

	// Check if the source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("[system] source file does not exist: %w", err)
	}

	// // Try rename first (fast path)
	// err := os.Rename(sourcePath, destPath)
	// if err == nil {
	// 	return nil
	// }

	// If rename fails, fallback to copy + delete
	golog.Debug("[system] Rename failed, falling back to copy+delete for: ", sourcePath)

	// Copy file
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("[system] failed to open source file: %w", err)
	}
	defer source.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("[system] failed to create destination file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return fmt.Errorf("[system] failed to copy file: %w", err)
	}

	// Close files before removing source
	source.Close()
	dest.Close()

	// Remove source file
	if err := os.Remove(sourcePath); err != nil {
		return fmt.Errorf("[system] failed to remove source file after copy: %w", err)
	}
	golog.Debug("[system] File moved successfully from ", sourcePath, " to ", destPath)

	return nil
}
