package gourdianlogger

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// LogLevel represents the severity levels for log messages in increasing order of severity.
// The logger will only output messages at or above the currently set log level.
//
// Levels:
//
//	DEBUG - Detailed debug information (most verbose)
//	INFO  - General operational messages about application flow
//	WARN  - Potentially harmful situations that aren't errors
//	ERROR - Error events that might still allow the application to continue
//	FATAL - Severe errors that will prevent the application from continuing (will call os.Exit(1))
//
// Example:
//
//	logger.SetLogLevel(gourdianlogger.INFO) // Will log INFO, WARN, ERROR, FATAL
type LogLevel int32

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// LogFormat specifies the output format for log messages.
//
// Supported formats:
//
//	FormatPlain - Human-readable text format with timestamp, level, message
//	FormatJSON  - Structured JSON format with additional metadata
//
// Example:
//
//	config := DefaultConfig()
//	config.Format = gourdianlogger.FormatJSON
type LogFormat int

const (
	FormatPlain LogFormat = iota
	FormatJSON
)

var (
	defaultMaxBytes        int64  = 10 * 1024 * 1024 // 10MB
	defaultBackupCount     int    = 5
	defaultTimestampFormat string = "2006-01-02 15:04:05.000000"
	defaultLogsDir         string = "logs"
)

// LoggerConfig contains all configurable parameters for the logger.
// Used when creating a new logger instance via NewGourdianLogger().
//
// Fields:
//
//	Filename        - Base name for log files (without extension)
//	MaxBytes        - Maximum log file size in bytes before rotation (default 10MB)
//	BackupCount     - Number of rotated log files to keep (default 5)
//	LogLevel        - Minimum log level to output (default DEBUG)
//	TimestampFormat - Custom timestamp format (default "2006-01-02 15:04:05.000000")
//	LogsDir         - Directory to store log files (default "logs")
//	EnableCaller    - Whether to include caller information (file:line:function)
//	BufferSize      - Size of async buffer (0 for synchronous logging)
//	AsyncWorkers    - Number of async worker goroutines (default 1)
//	Format          - Output format (plain or JSON)
//	FormatConfig    - Format-specific configuration
//	EnableFallback  - Whether to use stderr fallback on write errors
//	MaxLogRate      - Maximum logs per second (0 for unlimited)
//	CompressBackups - Whether to gzip rotated log files
//	RotationTime    - Time-based rotation interval (0 to disable)
//	SampleRate      - Log sampling rate (1 logs every N messages)
//	CallerDepth     - Number of stack frames to skip for caller info
//
// Example:
//
//	config := LoggerConfig{
//	    Filename: "myapp",
//	    MaxBytes: 50 * 1024 * 1024, // 50MB
//	    LogLevel: gourdianlogger.INFO,
//	    Format: gourdianlogger.FormatJSON,
//	    FormatConfig: FormatConfig{
//	        PrettyPrint: true,
//	    },
//	}
type LoggerConfig struct {
	Filename        string        `json:"filename"`         // Base filename for logs
	MaxBytes        int64         `json:"max_bytes"`        // Max file size before rotation
	BackupCount     int           `json:"backup_count"`     // Number of backups to keep
	LogLevelStr     string        `json:"log_level"`        // Log level as string (for config)
	TimestampFormat string        `json:"timestamp_format"` // Custom timestamp format
	LogsDir         string        `json:"logs_dir"`         // Directory for log files
	EnableCaller    bool          `json:"enable_caller"`    // Include caller info
	BufferSize      int           `json:"buffer_size"`      // Buffer size for async logging
	AsyncWorkers    int           `json:"async_workers"`    // Number of async workers
	FormatStr       string        `json:"format"`           // Format as string (for config)
	FormatConfig    FormatConfig  `json:"format_config"`    // Format-specific config
	EnableFallback  bool          `json:"enable_fallback"`  // Whether to use fallback logging
	MaxLogRate      int           `json:"max_log_rate"`     // Max logs per second (0 for unlimited)
	CompressBackups bool          `json:"compress_backups"` // Whether to gzip rotated logs
	RotationTime    time.Duration `json:"rotation_time"`    // Time-based rotation interval
	SampleRate      int           `json:"sample_rate"`      // Log sampling rate (1 in N)
	CallerDepth     int           `json:"caller_depth"`     // How many stack frames to skip
	LogLevel        LogLevel      `json:"-"`                // Minimum log level (internal use)
	Outputs         []io.Writer   `json:"-"`                // Additional outputs
	Format          LogFormat     `json:"-"`                // Log message format (internal use)
	ErrorHandler    func(error)   `json:"-"`                // Custom error handler
}

// FormatConfig contains additional formatting options.
//
// Fields:
//
//	PrettyPrint  - Whether to pretty-print JSON output with indentation
//	CustomFields - Additional key-value pairs to include in every log message
//
// Example:
//
//	FormatConfig{
//	    PrettyPrint: true,
//	    CustomFields: map[string]interface{}{
//	        "app": "myapp",
//	        "env": "production",
//	    },
//	}
type FormatConfig struct {
	PrettyPrint  bool                   `json:"pretty_print"`  // For human-readable JSON
	CustomFields map[string]interface{} `json:"custom_fields"` // Custom fields to include
}

// Logger is the main logging instance that provides thread-safe logging capabilities.
// It supports both synchronous and asynchronous logging, log rotation,
// multiple output destinations, and structured logging.
//
// Create an instance using:
//   - NewGourdianLogger()
//   - NewGourdianLoggerWithDefault()
//   - WithConfig()
type Logger struct {
	fileMu       sync.Mutex   // Protects file operations
	bufferPoolMu sync.Mutex   // Protects buffer pool access
	outputsMu    sync.RWMutex // Protects outputs slice

	level           atomic.Int32
	baseFilename    string
	maxBytes        int64
	backupCount     int
	file            *os.File
	multiWriter     io.Writer
	bufferPool      sync.Pool
	timestampFormat string
	outputs         []io.Writer
	logsDir         string
	closed          atomic.Bool
	rotateChan      chan struct{}
	rotateCloseChan chan struct{}
	wg              sync.WaitGroup
	enableCaller    bool
	asyncQueue      chan *logEntry
	asyncCloseChan  chan struct{}
	asyncWorkers    int
	format          LogFormat
	formatConfig    FormatConfig
	fallbackWriter  io.Writer
	errorHandler    func(error)
	rateLimiter     *rate.Limiter
	config          LoggerConfig
	dynamicLevelFn  func() LogLevel
}

type logEntry struct {
	level      LogLevel
	message    string
	callerInfo string
	fields     map[string]interface{}
}

func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[l]
}

// ParseLogLevel converts a string representation to a LogLevel.
// Case-insensitive. Supports both full names and abbreviations.
//
// Parameters:
//
//	level: String representation ("DEBUG", "INFO", "WARN", "ERROR", "FATAL")
//
// Returns:
//
//	LogLevel: Corresponding log level constant
//	error:    If the string doesn't match any level
//
// Example:
//
//	level, err := ParseLogLevel("info")
//	if err != nil {
//	    return fmt.Errorf("invalid log level: %w", err)
//	}
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

// DefaultConfig returns a LoggerConfig with sensible default values.
//
// Default values:
//
//	Filename:        "app"
//	MaxBytes:        10MB
//	BackupCount:     5
//	LogLevel:        DEBUG
//	TimestampFormat: "2006-01-02 15:04:05.000000"
//	LogsDir:         "logs"
//	EnableCaller:    true
//	BufferSize:      0 (synchronous)
//	AsyncWorkers:    1
//	Format:          FormatPlain
//	EnableFallback:  true
//	MaxLogRate:      0 (unlimited)
//	CompressBackups: false
//	RotationTime:    0 (disabled)
//	SampleRate:      1 (no sampling)
//	CallerDepth:     3
//
// Example:
//
//	config := DefaultConfig()
//	config.Filename = "myapp"
//	logger, err := NewGourdianLogger(config)
func DefaultConfig() LoggerConfig {
	return LoggerConfig{
		Filename:        "app",
		MaxBytes:        defaultMaxBytes,
		BackupCount:     defaultBackupCount,
		LogLevel:        DEBUG,
		TimestampFormat: defaultTimestampFormat,
		LogsDir:         defaultLogsDir,
		EnableCaller:    true,
		BufferSize:      0, // Sync by default
		AsyncWorkers:    1,
		Format:          FormatPlain,
		FormatConfig:    FormatConfig{},
		EnableFallback:  true,
		MaxLogRate:      0, // Unlimited by default
		CompressBackups: false,
		RotationTime:    0, // No time-based rotation by default
		SampleRate:      1, // No sampling by default
		CallerDepth:     3, // Default skip 3 frames
	}
}

// NewGourdianLogger creates a new logger instance with the specified configuration.
//
// Parameters:
//
//	config: Logger configuration (see LoggerConfig)
//
// Returns:
//
//	*Logger: The created logger instance
//	error:  Any error that occurred during initialization
//
// Example:
//
//	config := LoggerConfig{
//	    Filename: "myapp",
//	    LogLevel: gourdianlogger.INFO,
//	}
//	logger, err := NewGourdianLogger(config)
//	if err != nil {
//	    log.Fatal("Failed to create logger:", err)
//	}
//	defer logger.Close()
func NewGourdianLogger(config LoggerConfig) (*Logger, error) {
	// Apply environment variable overrides
	config.ApplyEnvOverrides()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Set defaults
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
	if config.Filename == "" {
		config.Filename = "app"
	}
	if config.AsyncWorkers <= 0 && config.BufferSize > 0 {
		config.AsyncWorkers = 1
	}
	if config.CallerDepth <= 0 {
		config.CallerDepth = 3
	}

	config.Filename = strings.TrimSpace(strings.TrimSuffix(config.Filename, ".log"))
	if err := os.MkdirAll(config.LogsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(config.LogsDir, config.Filename+".log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	outputs := []io.Writer{file, os.Stdout}
	if config.Outputs != nil {
		outputs = append(outputs, config.Outputs...)
	}
	validOutputs := make([]io.Writer, 0, len(outputs))
	for _, w := range outputs {
		if w != nil {
			validOutputs = append(validOutputs, w)
		}
	}

	logger := &Logger{
		baseFilename:    logPath,
		maxBytes:        config.MaxBytes,
		backupCount:     config.BackupCount,
		file:            file,
		timestampFormat: config.TimestampFormat,
		outputs:         validOutputs,
		logsDir:         config.LogsDir,
		rotateChan:      make(chan struct{}, 1),
		rotateCloseChan: make(chan struct{}),
		enableCaller:    config.EnableCaller,
		asyncWorkers:    config.AsyncWorkers,
		format:          config.Format,
		formatConfig:    config.FormatConfig,
		errorHandler:    config.ErrorHandler,
		config:          config,
	}

	// Initialize fallback writer if enabled
	if config.EnableFallback {
		logger.fallbackWriter = os.Stderr
	}

	// Initialize rate limiter if configured
	if config.MaxLogRate > 0 {
		logger.rateLimiter = rate.NewLimiter(rate.Limit(config.MaxLogRate), config.MaxLogRate)
	}

	logger.level.Store(int32(config.LogLevel))
	logger.multiWriter = io.MultiWriter(validOutputs...)
	logger.bufferPool.New = func() interface{} {
		return new(bytes.Buffer)
	}

	// Start file size-based rotation worker
	logger.wg.Add(1)
	go logger.fileSizeRotationWorker()

	// Start time-based rotation worker only if enabled
	if config.RotationTime > 0 {
		logger.wg.Add(1)
		go logger.timeRotationWorker(config.RotationTime)
	}

	// Initialize async logging if configured
	if config.BufferSize > 0 {
		logger.asyncQueue = make(chan *logEntry, config.BufferSize)
		logger.asyncCloseChan = make(chan struct{})

		for i := 0; i < logger.asyncWorkers; i++ {
			logger.wg.Add(1)
			go logger.asyncWorker()
		}
	}

	return logger, nil
}

// NewGourdianLoggerWithDefault creates a new logger with default configuration.
// This is a convenience function for quick initialization.
//
// Returns:
//
//	*Logger: Logger instance with default settings
//	error:  Any initialization error
//
// Example:
//
//	logger, err := NewGourdianLoggerWithDefault()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.Close()
func NewGourdianLoggerWithDefault() (*Logger, error) {
	config := DefaultConfig()
	return NewGourdianLogger(config)
}

// Validate checks the LoggerConfig for valid values
func (lc *LoggerConfig) Validate() error {
	if lc.MaxBytes < 0 {
		return fmt.Errorf("MaxBytes cannot be negative")
	}
	if lc.BackupCount < 0 {
		return fmt.Errorf("BackupCount cannot be negative")
	}
	if lc.BufferSize < 0 {
		return fmt.Errorf("BufferSize cannot be negative")
	}
	if lc.AsyncWorkers < 0 {
		return fmt.Errorf("AsyncWorkers cannot be negative")
	}
	if lc.MaxLogRate < 0 {
		return fmt.Errorf("MaxLogRate cannot be negative")
	}
	if lc.SampleRate < 1 {
		return fmt.Errorf("SampleRate must be at least 1")
	}
	if lc.CallerDepth < 1 {
		return fmt.Errorf("CallerDepth must be at least 1")
	}

	// Validate log level if specified
	if lc.LogLevelStr != "" {
		if _, err := ParseLogLevel(lc.LogLevelStr); err != nil {
			return fmt.Errorf("invalid log level: %w", err)
		}
	}

	// Validate format if specified
	if lc.FormatStr != "" {
		switch strings.ToUpper(lc.FormatStr) {
		case "PLAIN", "JSON":
			// valid
		default:
			return fmt.Errorf("invalid log format: %s", lc.FormatStr)
		}
	}

	return nil
}

// ApplyEnvOverrides applies environment variable overrides to the config
func (lc *LoggerConfig) ApplyEnvOverrides() {
	if dir := os.Getenv("LOG_DIR"); dir != "" {
		lc.LogsDir = dir
	}
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		lc.LogLevelStr = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		lc.FormatStr = format
	}
	if rate := os.Getenv("LOG_RATE"); rate != "" {
		if r, err := strconv.Atoi(rate); err == nil {
			lc.MaxLogRate = r
		}
	}
}

// fileSizeRotationWorker listens for manual rotation triggers
func (l *Logger) fileSizeRotationWorker() {
	defer l.wg.Done()
	for {
		select {
		case <-l.rotateChan:
			l.fileMu.Lock()
			if l.file != nil {
				if err := l.rotateLogFiles(); err != nil {
					l.handleError(fmt.Errorf("log rotation failed: %w", err))
				}
			}
			l.fileMu.Unlock()
		case <-l.rotateCloseChan:
			return
		}
	}
}

// timeRotationWorker triggers rotation at regular intervals
func (l *Logger) timeRotationWorker(interval time.Duration) {
	defer l.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			shouldRotate := false

			// Check if we should rotate without holding the lock
			l.fileMu.Lock()
			if l.file != nil {
				if l.config.RotationTime > 0 {
					shouldRotate = true
				}
			}
			l.fileMu.Unlock()

			if shouldRotate {
				// Perform rotation (which will take its own locks)
				if err := l.rotateLogFiles(); err != nil {
					// Use non-blocking error handling
					go l.handleError(fmt.Errorf("time-based log rotation failed: %w", err))
				}
			}
		case <-l.rotateCloseChan:
			return
		}
	}
}

// asyncWorker processes log entries asynchronously
func (l *Logger) asyncWorker() {
	defer l.wg.Done()

	// Batch processing variables
	batch := make([]*logEntry, 0, 100)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case entry := <-l.asyncQueue:
			batch = append(batch, entry)
			if len(batch) >= 100 {
				l.processBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.processBatch(batch)
				batch = batch[:0]
			}
		case <-l.asyncCloseChan:
			if len(batch) > 0 {
				l.processBatch(batch)
			}
			// Drain the queue before exiting
			for {
				select {
				case entry := <-l.asyncQueue:
					l.processLogEntry(entry.level, entry.message, entry.callerInfo, entry.fields)
				default:
					return
				}
			}
		}
	}
}

// processBatch processes a batch of log entries while holding the file lock
func (l *Logger) processBatch(entries []*logEntry) {
	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	for _, entry := range entries {
		currentLevel := l.GetLogLevel()
		if l.dynamicLevelFn != nil {
			currentLevel = l.dynamicLevelFn()
		}

		if entry.level < currentLevel {
			continue
		}

		// Apply sampling if configured
		if l.config.SampleRate > 1 && rand.Intn(l.config.SampleRate) != 0 {
			continue
		}

		// Get buffer with minimal locking
		l.bufferPoolMu.Lock()
		buf := l.bufferPool.Get().(*bytes.Buffer)
		l.bufferPoolMu.Unlock()

		buf.Reset()
		defer func() {
			l.bufferPoolMu.Lock()
			l.bufferPool.Put(buf)
			l.bufferPoolMu.Unlock()
		}()

		buf.WriteString(l.formatMessage(entry.level, entry.message, entry.callerInfo, entry.fields))

		if l.closed.Load() {
			fmt.Fprintf(os.Stderr, "Logger closed. Message: %s", buf.String())
			continue
		}

		if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
			l.handleError(fmt.Errorf("log write error: %w", err))
			if l.fallbackWriter != nil {
				fmt.Fprintf(l.fallbackWriter, "FALLBACK LOG: %s", buf.String())
			}
		}

		if entry.level == FATAL {
			os.Exit(1)
		}
	}
}

func (l *Logger) handleError(err error) {
	if l.errorHandler != nil {
		l.errorHandler(err)
	} else if l.fallbackWriter != nil {
		fmt.Fprintf(l.fallbackWriter, "LOGGER ERROR: %v\n", err)
	}
}

// processLogEntry processes a single log entry
func (l *Logger) processLogEntry(level LogLevel, message string, callerInfo string, fields map[string]interface{}) {
	currentLevel := l.GetLogLevel()
	if l.dynamicLevelFn != nil {
		currentLevel = l.dynamicLevelFn()
	}

	if level < currentLevel {
		return
	}

	// Apply sampling if configured
	if l.config.SampleRate > 1 && rand.Intn(l.config.SampleRate) != 0 {
		return
	}

	// Get buffer with minimal locking
	l.bufferPoolMu.Lock()
	buf := l.bufferPool.Get().(*bytes.Buffer)
	l.bufferPoolMu.Unlock()

	buf.Reset()
	defer func() {
		l.bufferPoolMu.Lock()
		l.bufferPool.Put(buf)
		l.bufferPoolMu.Unlock()
	}()

	buf.WriteString(l.formatMessage(level, message, callerInfo, fields))

	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	if l.closed.Load() {
		fmt.Fprintf(os.Stderr, "Logger closed. Message: %s", buf.String())
		return
	}

	if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
		l.handleError(fmt.Errorf("log write error: %w", err))
		if l.fallbackWriter != nil {
			fmt.Fprintf(l.fallbackWriter, "FALLBACK LOG: %s", buf.String())
		}
	}

	if level == FATAL {
		os.Exit(1)
	}
}

// formatPlain formats a log message in plain text
func (l *Logger) formatPlain(level LogLevel, message string, callerInfo string, fields map[string]interface{}) string {
	levelStr := fmt.Sprintf("[%s]", level.String())
	timestampStr := time.Now().Format(l.timestampFormat)

	var builder strings.Builder
	builder.WriteString(timestampStr)
	builder.WriteString(" ")
	builder.WriteString(levelStr)
	builder.WriteString(" ")

	if callerInfo != "" {
		builder.WriteString(callerInfo)
		builder.WriteString(":")
	}

	builder.WriteString(message)

	// Add fields if present
	if len(fields) > 0 {
		builder.WriteString(" {")
		first := true
		for k, v := range fields {
			if !first {
				builder.WriteString(", ")
			}
			first = false
			builder.WriteString(k)
			builder.WriteString("=")
			builder.WriteString(fmt.Sprintf("%v", v))
		}
		builder.WriteString("}")
	}

	builder.WriteString("\n")
	return builder.String()
}

// formatJSON formats a log message in JSON
func (l *Logger) formatJSON(level LogLevel, message string, callerInfo string, fields map[string]interface{}) string {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(l.timestampFormat),
		"level":     level.String(),
		"message":   message,
	}

	if l.enableCaller && callerInfo != "" {
		logEntry["caller"] = callerInfo
	}

	// Add custom fields from format config
	for k, v := range l.formatConfig.CustomFields {
		logEntry[k] = v
	}

	// Add log-specific fields
	for k, v := range fields {
		logEntry[k] = v
	}

	var jsonData []byte
	var err error

	if l.formatConfig.PrettyPrint {
		jsonData, err = json.MarshalIndent(logEntry, "", "  ")
	} else {
		jsonData, err = json.Marshal(logEntry)
	}

	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal log entry: %v"}`+"\n", err)
	}
	return string(jsonData) + "\n"
}

// getCallerInfo retrieves information about the caller
func (l *Logger) getCallerInfo(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}

	fileName := filepath.Base(file)
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%s:%d", fileName, line)
	}

	fullFnName := fn.Name()
	if lastSlash := strings.LastIndex(fullFnName, "/"); lastSlash >= 0 {
		fullFnName = fullFnName[lastSlash+1:]
	}

	return fmt.Sprintf("%s:%d:%s", fileName, line, fullFnName)
}

// formatMessage formats the log message according to the configured format
func (l *Logger) formatMessage(level LogLevel, message string, callerInfo string, fields map[string]interface{}) string {
	switch l.format {
	case FormatJSON:
		return l.formatJSON(level, message, callerInfo, fields)
	default:
		return l.formatPlain(level, message, callerInfo, fields)
	}
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, message string, skip int, fields map[string]interface{}) {
	// Apply rate limiting if configured
	if l.rateLimiter != nil && !l.rateLimiter.Allow() {
		return
	}

	var callerInfo string
	if l.enableCaller {
		callerInfo = l.getCallerInfo(skip)
	}

	if l.asyncQueue != nil {
		select {
		case l.asyncQueue <- &logEntry{level, message, callerInfo, fields}:
		default:
			// If queue is full, process synchronously
			l.processLogEntry(level, message, callerInfo, fields)
		}
	} else {
		l.processLogEntry(level, message, callerInfo, fields)
	}
}

// rotateLogFiles performs log file rotation
func (l *Logger) rotateLogFiles() error {
	t := time.Now()
	var rotationErr error

	// Move the debug log outside the locked section
	defer func() {
		if rotationErr == nil {
			go l.Debugf("rotateLogFiles completed in %v", time.Since(t))
		}
	}()

	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	if l.file == nil {
		rotationErr = fmt.Errorf("log file not open")
		return rotationErr
	}

	// Get current file info
	fileInfo, err := l.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Only skip rotation if both size threshold not met and rotation time not triggered
	if fileInfo.Size() < l.maxBytes && l.config.RotationTime <= 0 {
		return nil
	}

	// Close current file
	oldFile := l.file
	if err := oldFile.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	// Create rotated file name
	base := strings.TrimSuffix(l.baseFilename, ".log")
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_%s.log", base, timestamp)

	// Rename current file
	if err := os.Rename(l.baseFilename, backupPath); err != nil {
		// Try to reopen original file if rename failed
		file, openErr := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			return fmt.Errorf("failed to rename log file (%v) and couldn't reopen original (%v)", err, openErr)
		}
		l.file = file
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Open new log file
	file, err := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	// Update logger state
	l.file = file

	// Update outputs with minimal locking
	l.outputsMu.Lock()
	outputs := []io.Writer{os.Stdout, file}
	if len(l.outputs) > 2 {
		outputs = append(outputs, l.outputs[2:]...)
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)
	l.outputsMu.Unlock()

	// Compress backup if configured
	if l.config.CompressBackups {
		go func() {
			if err := compressFile(backupPath); err != nil {
				l.handleError(fmt.Errorf("failed to compress log file: %w", err))
			}
		}()
	}

	l.cleanupOldBackups()

	return rotationErr
}

// compressFile compresses a file using gzip
func compressFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(path + ".gz")
	if err != nil {
		return err
	}
	defer dst.Close()

	gz := gzip.NewWriter(dst)
	defer gz.Close()

	if _, err := io.Copy(gz, src); err != nil {
		return err
	}

	// Remove original file after successful compression
	return os.Remove(path)
}

// cleanupOldBackups removes old log backups exceeding the backup count
func (l *Logger) cleanupOldBackups() {
	pattern := strings.TrimSuffix(l.baseFilename, ".log") + "_*.log*"
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) <= l.backupCount {
		return
	}

	// Sort files by modification time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		info1, _ := os.Stat(files[i])
		info2, _ := os.Stat(files[j])
		return info1.ModTime().Before(info2.ModTime())
	})

	for _, f := range files[:len(files)-l.backupCount] {
		_ = os.Remove(f)
	}
}

// SetLogLevel dynamically changes the minimum log level at runtime.
// Messages below this level will be ignored.
//
// Parameters:
//
//	level: New minimum log level (DEBUG, INFO, WARN, ERROR, FATAL)
//
// Example:
//
//	// Only log warnings and above in production
//	if isProduction() {
//	    logger.SetLogLevel(gourdianlogger.WARN)
//	}
func (l *Logger) SetLogLevel(level LogLevel) {
	l.level.Store(int32(level))
}

// GetLogLevel returns the current minimum log level.
//
// Returns:
//
//	LogLevel: The current minimum log level
//
// Example:
//
//	if logger.GetLogLevel() == gourdianlogger.DEBUG {
//	    // Perform debug-specific actions
//	}
func (l *Logger) GetLogLevel() LogLevel {
	return LogLevel(l.level.Load())
}

// SetDynamicLevelFunc sets a function that dynamically determines the log level.
// The function is called for each log message to check if it should be logged.
// Useful for implementing runtime log level changes without locks.
//
// Parameters:
//
//	fn: Function that returns the current log level
//
// Example:
//
//	logger.SetDynamicLevelFunc(func() gourdianlogger.LogLevel {
//	    if isDebugMode() {
//	        return gourdianlogger.DEBUG
//	    }
//	    return gourdianlogger.INFO
//	})
func (l *Logger) SetDynamicLevelFunc(fn func() LogLevel) {
	l.dynamicLevelFn = fn
}

// AddOutput adds an additional output destination for log messages.
// Supports any io.Writer such as files, network connections, etc.
//
// Parameters:
//
//	output: The io.Writer to add as output
//
// Example:
//
//	// Log to both console and file
//	file, _ := os.Create("app.log")
//	logger.AddOutput(file)
func (l *Logger) AddOutput(output io.Writer) {
	if output == nil {
		return
	}

	l.outputsMu.Lock()
	defer l.outputsMu.Unlock()

	if l.closed.Load() {
		return
	}

	l.outputs = append(l.outputs, output)
	l.multiWriter = io.MultiWriter(l.outputs...)
}

// RemoveOutput removes an output destination from the logger.
//
// Parameters:
//
//	output: The io.Writer to remove
//
// Example:
//
//	logger.RemoveOutput(os.Stdout) // Stop logging to console
func (l *Logger) RemoveOutput(output io.Writer) {
	l.outputsMu.Lock()
	defer l.outputsMu.Unlock()

	if l.closed.Load() {
		return
	}

	for i, w := range l.outputs {
		if w == output {
			l.outputs = append(l.outputs[:i], l.outputs[i+1:]...)
			l.multiWriter = io.MultiWriter(l.outputs...)
			return
		}
	}
}

// Close gracefully shuts down the logger, ensuring all buffered logs are written.
// Should be called before application exit to prevent log loss.
//
// Returns:
//
//	error: Any error that occurred during shutdown
//
// Example:
//
//	defer logger.Close()
func (l *Logger) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}

	// Close channels first to stop workers
	close(l.rotateCloseChan)
	if l.asyncCloseChan != nil {
		close(l.asyncCloseChan)
	}

	// Wait for all workers to finish
	l.wg.Wait()

	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	// Close file last
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Debug logs a message at DEBUG level.
// Use for detailed debug information that is typically only needed during development.
// Messages are concatenated with spaces like fmt.Sprint.
//
// Parameters:
//
//	v: Values to log (any number of parameters, concatenated with spaces)
//
// Example:
//
//	logger.Debug("Current state:", state, "value:", value)
//	Output: "Current state: running value: 42"
func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

// Info logs a message at INFO level.
// Use for general operational messages about application flow.
//
// Parameters:
//
//	v: Values to log (concatenated with spaces)
//
// Example:
//
//	logger.Info("Server started on port", port)
func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

// Warn logs a message at WARN level.
// Use for potentially harmful situations that aren't errors.
//
// Parameters:
//
//	v: Values to log (concatenated with spaces)
//
// Example:
//
//	logger.Warn("Disk space below 10%")
func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

// Error logs a message at ERROR level.
// Use for error events that might still allow the application to continue.
//
// Parameters:
//
//	v: Values to log (concatenated with spaces)
//
// Example:
//
//	logger.Error("Failed to connect to database:", err)
func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

// Fatal logs a message at FATAL level and exits the program with status 1.
// Use for severe errors that prevent the application from continuing.
// Automatically flushes any buffered logs before exiting.
//
// Parameters:
//
//	v: Values to log (concatenated with spaces)
//
// Example:
//
//	logger.Fatal("Cannot start - required service unavailable")
func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), l.config.CallerDepth, nil)
	l.Flush()
	os.Exit(1)
}

// Debugf logs a formatted message at DEBUG level.
// Supports all formatting verbs from fmt.Sprintf.
//
// Parameters:
//
//	format: Format string (supports %v, %s, %d etc.)
//	v:      Values to interpolate into the format string
//
// Example:
//
//	logger.Debugf("User %s (ID: %d) logged in", username, userID)
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

// Infof logs a formatted message at INFO level.
//
// Parameters:
//
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.Infof("Processing %d records took %v", count, duration)
func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

// Warnf logs a formatted message at WARN level.
//
// Parameters:
//
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.Warnf("High latency detected: %.2fms", latencyMs)
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

// Errorf logs a formatted message at ERROR level.
//
// Parameters:
//
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.Errorf("Failed to process request: %v", err)
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

// Fatalf logs a formatted message at FATAL level and exits the program.
// Automatically flushes any buffered logs before exiting.
//
// Parameters:
//
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.Fatalf("Critical error: %v - shutting down", criticalErr)
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
	l.Flush()
	os.Exit(1)
}

// DebugWithFields logs a message with additional structured fields at DEBUG level.
// Fields are included in both plain and JSON formats.
//
// Parameters:
//
//	fields: Key-value pairs of additional context (map[string]interface{})
//	v:      Values to log (concatenated with spaces)
//
// Example:
//
//	logger.DebugWithFields(map[string]interface{}{
//	    "user":    "john",
//	    "action":  "login",
//	    "success": true,
//	}, "Authentication attempt")
func (l *Logger) DebugWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

// InfoWithFields logs a message with additional fields at INFO level.
//
// Parameters:
//
//	fields: Additional context as key-value pairs
//	v:      Values to log
//
// Example:
//
//	logger.InfoWithFields(map[string]interface{}{
//	    "request_id": reqID,
//	    "duration":   duration,
//	}, "Request processed")
func (l *Logger) InfoWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

// WarnWithFields logs a message with additional fields at WARN level.
//
// Parameters:
//
//	fields: Additional context
//	v:      Values to log
//
// Example:
//
//	logger.WarnWithFields(map[string]interface{}{
//	    "threshold": 90,
//	    "current":   usage,
//	}, "Resource usage high")
func (l *Logger) WarnWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

// ErrorWithFields logs a message with additional fields at ERROR level.
//
// Parameters:
//
//	fields: Additional context
//	v:      Values to log
//
// Example:
//
//	logger.ErrorWithFields(map[string]interface{}{
//	    "function": "processPayment",
//	    "order_id": orderID,
//	}, "Payment processing failed")
func (l *Logger) ErrorWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

// FatalWithFields logs a message with additional fields at FATAL level and exits.
// Automatically flushes any buffered logs before exiting.
//
// Parameters:
//
//	fields: Additional context
//	v:      Values to log
//
// Example:
//
//	logger.FatalWithFields(map[string]interface{}{
//	    "error_code": 5001,
//	    "component": "database",
//	}, "Critical database failure")
func (l *Logger) FatalWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), l.config.CallerDepth, fields)
	l.Flush()
	os.Exit(1)
}

// DebugfWithFields combines formatted messages with structured logging at DEBUG level.
//
// Parameters:
//
//	fields: Additional context as key-value pairs
//	format: Format string (supports %v, %s etc.)
//	v:      Values to interpolate into format string
//
// Example:
//
//	logger.DebugfWithFields(map[string]interface{}{
//	    "duration_ms": 45,
//	    "method":      "GET",
//	}, "Request processed in %dms", 45)
func (l *Logger) DebugfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

// InfofWithFields combines formatted messages with structured logging at INFO level.
//
// Parameters:
//
//	fields: Additional context
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.InfofWithFields(map[string]interface{}{
//	    "user_count": count,
//	}, "Processed %d users", count)
func (l *Logger) InfofWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

// WarnfWithFields combines formatted messages with structured logging at WARN level.
//
// Parameters:
//
//	fields: Additional context
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.WarnfWithFields(map[string]interface{}{
//	    "threshold": 90,
//	}, "CPU usage at %d%%", usage)
func (l *Logger) WarnfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

// ErrorfWithFields combines formatted messages with structured logging at ERROR level.
//
// Parameters:
//
//	fields: Additional context
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.ErrorfWithFields(map[string]interface{}{
//	    "request_id": reqID,
//	}, "Failed to process request %s", reqID)
func (l *Logger) ErrorfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

// FatalfWithFields combines formatted messages with structured logging at FATAL level and exits.
// Automatically flushes any buffered logs before exiting.
//
// Parameters:
//
//	fields: Additional context
//	format: Format string
//	v:      Values to interpolate
//
// Example:
//
//	logger.FatalfWithFields(map[string]interface{}{
//	    "error_code": 5001,
//	}, "Critical error %d occurred", errCode)
func (l *Logger) FatalfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
	l.Flush()
	os.Exit(1)
}

// Flush ensures all buffered logs are written to output.
// Only needed when using async logging (BufferSize > 0).
// Automatically called by Close().
//
// Example:
//
//	// Before exiting ensure all logs are written
//	logger.Flush()
func (l *Logger) Flush() {
	if l.asyncQueue != nil {
		for len(l.asyncQueue) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// IsClosed checks if the logger has been closed.
//
// Returns:
//
//	bool: true if the logger is closed, false otherwise
//
// Example:
//
//	if !logger.IsClosed() {
//	    logger.Info("Application shutting down")
//	    logger.Close()
//	}
func (l *Logger) IsClosed() bool {
	return l.closed.Load()
}

// WithConfig creates a logger from a JSON configuration string.
// Useful for loading configuration from files or environment variables.
//
// Parameters:
//
//	jsonConfig: JSON string containing logger configuration
//
// Returns:
//
//	*Logger: New logger instance
//	error:  Any parsing or initialization error
//
// Example:
//
//	const configJSON = `{
//	    "filename": "myapp",
//	    "log_level": "info",
//	    "max_bytes": 5000000,
//	    "format": "json"
//	}`
//	logger, err := WithConfig(configJSON)
func WithConfig(jsonConfig string) (*Logger, error) {
	if !json.Valid([]byte(jsonConfig)) {
		return nil, fmt.Errorf("invalid JSON config")
	}

	var config LoggerConfig
	if err := json.Unmarshal([]byte(jsonConfig), &config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if config.LogsDir == "" {
		config.LogsDir = defaultLogsDir
	}

	if err := os.MkdirAll(config.LogsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory '%s': %w", config.LogsDir, err)
	}

	return NewGourdianLogger(config)
}

func (lc *LoggerConfig) UnmarshalJSON(data []byte) error {
	type Alias LoggerConfig
	aux := &struct {
		LogLevelStr string `json:"log_level"`
		FormatStr   string `json:"format"`
		*Alias
	}{
		Alias: (*Alias)(lc),
	}

	if string(data) == "{}" {
		*lc = DefaultConfig()
		return nil
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.LogLevelStr == "" {
		lc.LogLevel = DEBUG
	} else {
		level, err := ParseLogLevel(aux.LogLevelStr)
		if err != nil {
			return err
		}
		lc.LogLevel = level
	}

	if aux.FormatStr == "" {
		lc.Format = FormatPlain
	} else {
		switch strings.ToUpper(aux.FormatStr) {
		case "PLAIN":
			lc.Format = FormatPlain
		case "JSON":
			lc.Format = FormatJSON
		default:
			return fmt.Errorf("invalid log format: %s", aux.FormatStr)
		}
	}

	// Set defaults for required fields if not provided
	if lc.CallerDepth == 0 {
		lc.CallerDepth = 3 // Default value
	}
	if lc.SampleRate < 1 {
		lc.SampleRate = 1
	}

	return nil
}
