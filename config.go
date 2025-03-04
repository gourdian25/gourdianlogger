package gourdianlogger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type LoggerConfig struct {
	Filename        string
	MaxBytes        int64
	BackupCount     int
	LogLevel        LogLevel
	TimestampFormat string
	Outputs         []io.Writer
}

func NewGourdianLogger(config LoggerConfig) (*Logger, error) {
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	if config.Filename == "./configs/gourdian.config.yml" {
		config.Filename = "gourdianlogger"
	}

	config.Filename = strings.TrimSuffix(config.Filename, ".log")
	logFilePath := filepath.Join(logsDir, config.Filename+".log")

	if config.MaxBytes == 0 {
		config.MaxBytes = 10 * 1024 * 1024 // 10MB
	}
	if config.BackupCount == 0 {
		config.BackupCount = 5
	}
	if config.TimestampFormat == "" {
		config.TimestampFormat = "2006/01/02 15:04:05.000000"
	}

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	outputs := []io.Writer{os.Stdout, file}
	if config.Outputs != nil {
		outputs = append(outputs, config.Outputs...)
	}
	multiWriter := io.MultiWriter(outputs...)

	logger := &Logger{
		baseFilename:    logFilePath,
		maxBytes:        config.MaxBytes,
		backupCount:     config.BackupCount,
		file:            file,
		level:           config.LogLevel,
		multiWriter:     multiWriter,
		timestampFormat: config.TimestampFormat,
		outputs:         outputs,
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}

	return logger, nil
}
