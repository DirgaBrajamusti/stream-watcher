package common

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// OutputParser is implemented by downloaders that want to parse process output.
type OutputParser interface {
	// Parse processes a single line of output. The update callback accepts a mutation
	// function that is applied to the job while holding the store lock. This is the only
	// safe way to mutate the job while it may be read concurrently by the web server.
	Parse(line string, update func(func(*DownloadJob)))
}

// DownloaderConfig describes how to invoke an external download executable.
type DownloaderConfig struct {
	Executable string
	WorkingDir string
	Args       []string
}

// RunExternalDownloader spawns an external download process and blocks until it exits.
// stdout and stderr are parsed line-by-line by the provided parser and are also
// written to a per-job log file in the job's output directory.
func RunExternalDownloader(ctx context.Context, cfg DownloaderConfig, job *DownloadJob, parser OutputParser) error {
	cmd := buildCmd(ctx, cfg)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	logFile, err := openOutputLog(job)
	if err != nil {
		slog.Warn("download: failed to open output log file", "video_id", job.VideoID, "error", err)
	} else {
		defer logFile.Close()
	}

	var logMu sync.Mutex
	writeLog := func(line string) {
		if logFile == nil {
			return
		}
		logMu.Lock()
		defer logMu.Unlock()
		if _, err := fmt.Fprintln(logFile, line); err != nil {
			slog.Debug("download: failed to write output log", "video_id", job.VideoID, "error", err)
		}
	}

	slog.Info("starting download", "video_id", job.VideoID, "executable", cfg.Executable)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", cfg.Executable, err)
	}

	update := func(fn func(*DownloadJob)) {
		DefaultStore.Update(job.VideoID, fn)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ReadOutputLines(stdout, func(line string) {
			writeLog(line)
			parser.Parse(line, update)
		})
	}()
	go func() {
		defer wg.Done()
		ReadOutputLines(stderr, func(line string) {
			writeLog(line)
			parser.Parse(line, update)
		})
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		// Non-zero exit after a successful finish is often still a success for ytarchive
		// when it has already reported "Finished". Do not overwrite a finished state.
		update(func(j *DownloadJob) {
			if j.Status != StatusFinished && j.Status != StatusProcessed {
				j.Status = StatusError
				j.Output = err.Error()
			}
		})
		return fmt.Errorf("process exited: %w", err)
	}

	return nil
}

// openOutputLog creates (or appends to) a per-job log file in the app's centralized
// ./logs directory, named <date>_<channel>_<videoID>.log. It returns an error only
// if the file cannot be created; output logging is skipped when it fails.
func openOutputLog(job *DownloadJob) (*os.File, error) {
	dir := "logs"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs directory: %w", err)
	}

	date := job.CreatedAt.Format("2006-01-02")
	channel := sanitizeFilename(job.ChannelLive.ChannelName)
	if channel == "" {
		channel = "unknown"
	}
	videoID := sanitizeFilename(job.VideoID)
	if videoID == "" {
		videoID = "unknown"
	}

	filename := fmt.Sprintf("%s_%s_%s.log", date, channel, videoID)
	return os.OpenFile(filepath.Join(dir, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// sanitizeFilename replaces characters that are invalid in filenames.
func sanitizeFilename(s string) string {
	return strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	).Replace(s)
}

func buildCmd(ctx context.Context, cfg DownloaderConfig) *exec.Cmd {
	var cmd *exec.Cmd
	args := append([]string(nil), cfg.Args...)

	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/C", cfg.Executable}, args...)
		cmd = exec.CommandContext(ctx, "cmd", cmdArgs...)
	} else {
		cmd = exec.CommandContext(ctx, cfg.Executable, args...)
	}
	cmd.Dir = cfg.WorkingDir

	slog.Debug("spawning command", "command", strings.Join(cmd.Args, " "), "dir", cfg.WorkingDir)
	return cmd
}
