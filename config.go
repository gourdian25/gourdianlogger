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

// LoggerConfig defines the configuration parameters for the Gourdian logger.
//
// This struct allows configuration of:
// - Log file location and name
// - Maximum file size before rotation
// - Number of backup files to retain
// - Minimum log level to record
// - Custom timestamp format
// - Additional outputs for logging
//
// Fields:
//   - Filename: Name of the log file (if empty, defaults to "gourdianlogger")
//   - MaxBytes: Maximum size of log file before rotation (in bytes)
//   - BackupCount: Number of rotated log files to keep
//   - LogLevel: Minimum level of logs to record (DEBUG to FATAL)
//   - TimestampFormat: Custom timestamp format (default: "2006/01/02 15:04:05.000000")
//   - Outputs: Additional outputs for logging (e.g., external services)
//
// Example:
//
//	config := LoggerConfig{
//	    Filename:        "myapp.log",
//	    MaxBytes:        10*1024*1024,  // 10MB
//	    BackupCount:     5,
//	    LogLevel:        INFO,
//	    TimestampFormat: "2006-01-02 15:04:05",
//	    Outputs:         []io.Writer{externalServiceWriter},
//	}
type LoggerConfig struct {
	Filename        string
	MaxBytes        int64
	BackupCount     int
	LogLevel        LogLevel
	TimestampFormat string
	Outputs         []io.Writer
}

// NewGourdianLogger creates a new rotating file logger with console output and advanced configuration.
//
// This function:
// - Creates a 'logs' directory if it doesn't exist
// - Handles special case for config file name
// - Sets up both file and console logging via MultiWriter
// - Configures log rotation with size limits
// - Initializes thread-safe logging with buffer pool
//
// Parameters:
//   - config: LoggerConfig struct containing logger settings
//
// Returns:
//   - *Logger: Configured logger instance
//   - error: Any error encountered during setup
//
// Default values:
//   - MaxBytes: 10MB if not specified
//   - BackupCount: 5 if not specified
//   - Filename: "gourdianlogger" if config filename is "./configs/gourdian.config.yml"
//   - TimestampFormat: "2006/01/02 15:04:05.000000" if not specified
//
// Example:
//
//	logger, err := NewGourdianLogger(LoggerConfig{
//	    Filename:    "myapp.log",
//	    MaxBytes:    10*1024*1024,
//	    BackupCount: 5,
//	    LogLevel:    INFO,
//	})
//	if err != nil {
//	    panic(err)
//	}
//	defer logger.Close()
//
//	logger.Info("Logger initialized successfully")
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

	// Combine console, file, and additional outputs
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
