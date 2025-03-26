package gourdianlogger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity level of log messages
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var (
	// Default values
	defaultMaxBytes        int64 = 10 * 1024 * 1024 // 10MB
	defaultBackupCount           = 5
	defaultTimestampFormat       = "2006/01/02 15:04:05.000000"
	defaultLogsDir               = "logs"
)

// String returns the string representation of the LogLevel
func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[l]
}

// ParseLogLevel converts a string to a LogLevel
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN", "WARNING":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	default:
		return DEBUG, fmt.Errorf("invalid log level: %s", level)
	}
}

// NewLoggerConfig creates a new LoggerConfig with all parameters.
// This is the full constructor that requires all fields to be specified.
func NewLoggerConfig(
	filename string,
	maxBytes int64,
	backupCount int,
	logLevel LogLevel,
	timestampFormat string,
	outputs []io.Writer,
	logsDir string,
) LoggerConfig {
	return LoggerConfig{
		Filename:        filename,
		MaxBytes:        maxBytes,
		BackupCount:     backupCount,
		LogLevel:        logLevel,
		TimestampFormat: timestampFormat,
		Outputs:         outputs,
		LogsDir:         logsDir,
	}
}

// DefaultLoggerConfig returns a LoggerConfig with default values.
// This is useful when you want to start with sensible defaults and override specific fields.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Filename:        "gourdianlogger",
		MaxBytes:        defaultMaxBytes, // 10MB
		BackupCount:     defaultBackupCount,
		LogLevel:        DEBUG,
		TimestampFormat: defaultTimestampFormat,
		Outputs:         nil,
		LogsDir:         defaultLogsDir,
	}
}

// NewLoggerConfigWithDefaults creates a new LoggerConfig with some parameters and defaults for others.
// This provides flexibility to specify only the fields you care about.
func NewLoggerConfigWithDefaults(
	filename string,
	logLevel LogLevel,
	outputs []io.Writer,
) LoggerConfig {
	return LoggerConfig{
		Filename:        filename,
		MaxBytes:        defaultMaxBytes,
		BackupCount:     defaultBackupCount,
		LogLevel:        logLevel,
		TimestampFormat: defaultTimestampFormat,
		Outputs:         outputs,
		LogsDir:         defaultLogsDir,
	}
}

// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	Filename        string      // Base filename for logs (without extension)
	MaxBytes        int64       // Maximum size before rotation (default 10MB)
	BackupCount     int         // Number of backups to keep (default 5)
	LogLevel        LogLevel    // Minimum log level (default DEBUG)
	TimestampFormat string      // Custom timestamp format
	Outputs         []io.Writer // Additional output destinations
	LogsDir         string      // Directory to store log files (default "logs")
}

// Logger is the main logging struct
type Logger struct {
	mu              sync.RWMutex   // Ensures thread-safe operations
	level           LogLevel       // Minimum log level to record
	baseFilename    string         // Base name of the log file
	maxBytes        int64          // Maximum size before rotation
	backupCount     int            // Number of backups to keep
	file            *os.File       // Current log file handle
	multiWriter     io.Writer      // Writes to multiple outputs
	bufferPool      *sync.Pool     // Pool of reusable buffers
	timestampFormat string         // Timestamp format
	outputs         []io.Writer    // Additional outputs
	logsDir         string         // Directory for log files
	closed          bool           // Track if logger is closed
	rotateChan      chan struct{}  // Channel for rotation signals
	rotateCloseChan chan struct{}  // Channel to stop rotation goroutine
	wg              sync.WaitGroup // Wait group for background operations
}

// NewGourdianLogger creates a new logger instance
func NewGourdianLogger(config LoggerConfig) (*Logger, error) {
	// Apply defaults
	if config.MaxBytes <= 0 {
		config.MaxBytes = defaultMaxBytes
	}
	if config.BackupCount <= 0 {
		config.BackupCount = defaultBackupCount
	}
	if config.TimestampFormat == "" {
		config.TimestampFormat = defaultTimestampFormat
	}
	if config.LogsDir == "" {
		config.LogsDir = defaultLogsDir
	}

	// Ensure logs directory exists
	if err := os.MkdirAll(config.LogsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Clean and prepare filename
	config.Filename = strings.TrimSpace(config.Filename)
	if config.Filename == "" {
		config.Filename = "gourdianlogger"
	}
	config.Filename = strings.TrimSuffix(config.Filename, ".log")

	logFilePath := filepath.Join(config.LogsDir, config.Filename+".log")

	// Open log file
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Prepare outputs
	outputs := []io.Writer{os.Stdout, file}
	if config.Outputs != nil {
		outputs = append(outputs, config.Outputs...)
	}

	logger := &Logger{
		baseFilename:    logFilePath,
		maxBytes:        config.MaxBytes,
		backupCount:     config.BackupCount,
		file:            file,
		level:           config.LogLevel,
		multiWriter:     io.MultiWriter(outputs...),
		timestampFormat: config.TimestampFormat,
		outputs:         outputs,
		logsDir:         config.LogsDir,
		rotateChan:      make(chan struct{}, 1),
		rotateCloseChan: make(chan struct{}),
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}

	// Start background rotation checker
	logger.wg.Add(1)
	go logger.rotationChecker()

	return logger, nil
}

// SetLogLevel changes the minimum log level
func (l *Logger) SetLogLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLogLevel returns the current log level
func (l *Logger) GetLogLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// AddOutput adds a new output destination
func (l *Logger) AddOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}

	l.outputs = append(l.outputs, output)
	l.multiWriter = io.MultiWriter(l.outputs...)
}

// RemoveOutput removes an output destination
func (l *Logger) RemoveOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}

	for i, w := range l.outputs {
		if w == output {
			l.outputs = append(l.outputs[:i], l.outputs[i+1:]...)
			break
		}
	}
	l.multiWriter = io.MultiWriter(l.outputs...)
}

// formatLogMessage formats a log message with timestamp, level, caller info
func (l *Logger) formatLogMessage(level LogLevel, message string) string {
	// Get caller info (4 levels up the stack)
	pc, file, line, ok := runtime.Caller(4)
	if !ok {
		file = "unknown"
		line = 0
	}

	// Extract function name
	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		fullFunc := fn.Name()
		// Simplify package path
		if lastSlash := strings.LastIndex(fullFunc, "/"); lastSlash >= 0 {
			fullFunc = fullFunc[lastSlash+1:]
		}
		// Extract just the function name
		if lastDot := strings.LastIndex(fullFunc, "."); lastDot >= 0 {
			funcName = fullFunc[lastDot+1:]
		} else {
			funcName = fullFunc
		}
	}

	// Simplify file path
	file = filepath.Base(file)

	// Format the message
	return fmt.Sprintf("%s [%s] %s:%d (%s): %s\n",
		time.Now().Format(l.timestampFormat),
		level,
		file,
		line,
		funcName,
		message,
	)
}

// log is the internal logging function
func (l *Logger) log(level LogLevel, message string) {
	// Check log level first without lock for performance
	if level < l.GetLogLevel() {
		return
	}

	// Format the message
	formattedMsg := l.formatLogMessage(level, message)

	// Get buffer from pool
	buf := l.bufferPool.Get().(*bytes.Buffer)
	defer l.bufferPool.Put(buf)
	buf.Reset()
	buf.WriteString(formattedMsg)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if logger is closed
	if l.closed {
		fmt.Fprintf(os.Stderr, "Logger is closed. Message: %s", formattedMsg)
		return
	}

	// Check file size and rotate if needed
	if err := l.checkFileSize(); err != nil {
		fmt.Fprintf(os.Stderr, "Log rotation error: %v\n", err)
	}

	// Write to all outputs
	if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log message: %v\nOriginal message: %s", err, formattedMsg)
	}

	// For FATAL logs, exit after writing
	if level == FATAL {
		os.Exit(1)
	}
}

// checkFileSize checks if the log file needs rotation
func (l *Logger) checkFileSize() error {
	fi, err := l.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fi.Size() >= l.maxBytes {
		// Signal rotation through channel (non-blocking)
		select {
		case l.rotateChan <- struct{}{}:
		default:
			// Rotation already pending
		}
	}

	return nil
}

// rotationChecker runs in background to handle rotations
func (l *Logger) rotationChecker() {
	defer l.wg.Done()

	for {
		select {
		case <-l.rotateChan:
			l.mu.Lock()
			if err := l.rotateLogFiles(); err != nil {
				fmt.Fprintf(os.Stderr, "Log rotation failed: %v\n", err)
			}
			l.mu.Unlock()
		case <-l.rotateCloseChan:
			return
		}
	}
}

// rotateLogFiles performs log rotation
func (l *Logger) rotateLogFiles() error {
	// Close current file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	// Generate new filename with timestamp
	baseWithoutExt := strings.TrimSuffix(l.baseFilename, ".log")
	timestamp := time.Now().Format("20060102_150405")
	newLogFilePath := fmt.Sprintf("%s_%s.log", baseWithoutExt, timestamp)

	// Rename current log file
	if err := os.Rename(l.baseFilename, newLogFilePath); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Get list of backup files
	backupFiles, err := filepath.Glob(baseWithoutExt + "_*.log")
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	// Sort backups by name (which includes timestamp)
	sort.Strings(backupFiles)

	// Remove oldest backups if we have too many
	if len(backupFiles) > l.backupCount {
		for _, oldFile := range backupFiles[:len(backupFiles)-l.backupCount] {
			if err := os.Remove(oldFile); err != nil {
				return fmt.Errorf("failed to remove old log file %s: %w", oldFile, err)
			}
		}
	}

	// Create new log file
	file, err := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	// Update logger state
	l.file = file

	// Rebuild outputs with new file
	outputs := []io.Writer{os.Stdout, file}
	if len(l.outputs) > 2 {
		outputs = append(outputs, l.outputs[2:]...)
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)

	return nil
}

// Logging methods

func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...))
}

func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...))
}

func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...))
}

func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...))
}

func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...))
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...))
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...))
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...))
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...))
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...))
}

// Close cleanly shuts down the logger
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	// Stop rotation goroutine
	close(l.rotateCloseChan)
	l.wg.Wait()

	// Close the log file
	err := l.file.Close()
	l.closed = true

	return err
}
