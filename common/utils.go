package common

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// CopyFile copies a file from src to dst. The destination directory is created if it
// does not exist.
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}
	return out.Close()
}

// MoveFile moves a file from sourcePath to destPath. It attempts os.Rename first for
// atomic moves within the same filesystem, and falls back to copy+delete for cross-
// filesystem moves.
func MoveFile(sourcePath, destPath string) error {
	slog.Debug("moving file", "from", sourcePath, "to", destPath)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("source file does not exist: %w", err)
	}

	if err := os.Rename(sourcePath, destPath); err == nil {
		return nil
	}

	if err := CopyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	if err := os.Remove(sourcePath); err != nil {
		return fmt.Errorf("remove source file: %w", err)
	}
	return nil
}

// MkdirAll ensures the given directories exist.
func MkdirAll(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

// ReadOutputLines reads lines from r and passes them to handler. It returns when the
// reader is closed or the context is cancelled.
func ReadOutputLines(r io.Reader, handler func(line string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		handler(scanner.Text())
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		slog.Debug("output reader error", "error", err)
	}
}
