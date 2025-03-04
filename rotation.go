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

	backupFiles, _ := filepath.Glob(baseWithoutExt + "_*.log")
	sort.Strings(backupFiles)

	if len(backupFiles) > l.backupCount {
		for _, oldFile := range backupFiles[:len(backupFiles)-l.backupCount] {
			os.Remove(oldFile)
		}
	}

	file, err := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.file = file

	outputs := []io.Writer{os.Stdout, file}
	if len(l.outputs) > 0 {
		outputs = append(outputs, l.outputs[2:]...)
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)

	return nil
}
