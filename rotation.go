package gourdianlogger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// checkFileSize checks if the current log file has exceeded the size limit.
//
// Returns:
//   - error: Any error encountered during size check or rotation
//
// Thread safety:
//
//	Caller must hold lock
func (l *Logger) checkFileSize() error {
	fi, err := l.file.Stat()
	if err != nil {
		return err
	}

	if fi.Size() >= l.maxBytes {
		return l.rotateLogFiles()
	}

	return nil
}

// rotateLogFiles performs log file rotation when size limit is reached.
//
// This method:
// - Closes current log file
// - Renames current file with timestamp
// - Creates new log file
// - Cleans up old backup files
// - Reinitializes multiWriter
//
// Returns:
//   - error: Any error encountered during rotation
//
// Thread safety:
//
//	Caller must hold lock
func (l *Logger) rotateLogFiles() error {
	if err := l.file.Close(); err != nil {
		return err
	}

	baseWithoutExt := strings.TrimSuffix(l.baseFilename, ".log")
	currentTime := time.Now().Format("20060102_150405")
	newLogFilePath := fmt.Sprintf("%s_%s.log", baseWithoutExt, currentTime)

	if err := os.Rename(l.baseFilename, newLogFilePath); err != nil {
		return err
	}

	// Get all backup files
	backupFiles, _ := filepath.Glob(baseWithoutExt + "_*.log")

	// Sort them to ensure we remove the oldest ones
	sort.Strings(backupFiles)

	// If we have more backups than allowed, remove the oldest ones
	if len(backupFiles) > l.backupCount {
		for _, oldFile := range backupFiles[:len(backupFiles)-l.backupCount] {
			os.Remove(oldFile)
		}
	}

	// Create new log file
	file, err := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.file = file

	// Reconstruct the multiWriter with all outputs
	outputs := []io.Writer{os.Stdout, file}
	if len(l.outputs) > 0 {
		outputs = append(outputs, l.outputs[2:]...) // Skip stdout and previous file
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)

	return nil
}
