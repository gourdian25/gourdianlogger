// File: gourdianlogger.go

package gourdianlogger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// LogLevel represents the severity level of a log message.
// The levels are ordered from least to most severe:
// DEBUG < INFO < WARN < ERROR < FATAL.
type LogLevel int32

// Constants defining the available log levels.
const (
	DEBUG LogLevel = iota // Debug-level messages for development
	INFO                  // Informational messages about normal operation
	WARN                  // Warning messages about potential issues
	ERROR                 // Error messages about problems that need attention
	FATAL                 // Fatal messages about critical errors that force shutdown
)

// LogFormat represents the format in which logs will be written.
type LogFormat int

// Constants defining available log formats.
const (
	FormatPlain LogFormat = iota // Plain text format (human-readable)
	FormatJSON                   // JSON format (machine-readable)
)

var (
	defaultBackupCount           = 5
	defaultBufferSize            = 0
	defaultAsyncWorkers          = 1
	defaultMaxLogRate            = 0
	defaultEnableCaller          = true
	defaultEnableFallback        = true
	defaultLogsDir               = "logs"
	defaultLogLevel              = DEBUG
	defaultLogFormat             = FormatPlain
	defaultFileName              = "gourdianlogs"
	defaultMaxBytes        int64 = 10 * 1024 * 1024
	defaultTimestampFormat       = "2006-01-02 15:04:05.000000"
)

// LoggerConfig contains configuration options for the logger.
// All fields have sensible defaults and are optional.
type LoggerConfig struct {
	BackupCount     int                    `json:"backup_count"`     // Number of backup log files to keep (default: 5)
	BufferSize      int                    `json:"buffer_size"`      // Size of the async buffer in messages (0 for synchronous logging, default: 0)
	AsyncWorkers    int                    `json:"async_workers"`    // Number of async worker goroutines (default: 1)
	MaxLogRate      int                    `json:"max_log_rate"`     // Maximum number of logs per second (0 for no limit, default: 0)
	EnableCaller    bool                   `json:"enable_caller"`    // Whether to include caller information (file:line:function) (default: true)
	EnableFallback  bool                   `json:"enable_fallback"`  // Whether to fallback to stderr when logging fails (default: true)
	PrettyPrint     bool                   `json:"pretty_print"`     // Pretty-print JSON logs (only applies to JSON format, default: false)
	MaxBytes        int64                  `json:"max_bytes"`        // Maximum size in bytes before rotating log file (default: 10MB)
	Filename        string                 `json:"filename"`         // Base filename for logs (without extension, default: "gourdianlogs")
	TimestampFormat string                 `json:"timestamp_format"` // Timestamp format (default: "2006-01-02 15:04:05.000000")
	LogsDir         string                 `json:"logs_dir"`         // Directory to store log files (default: "logs")
	FormatStr       string                 `json:"format"`           // Format string ("PLAIN" or "JSON", default: "PLAIN")
	LogLevel        LogLevel               `json:"log_level"`        // Default log level (default: DEBUG)
	CustomFields    map[string]interface{} `json:"custom_fields"`    // Custom fields to include in every log message (JSON format only)
	Outputs         []io.Writer            `json:"-"`                // Additional output writers (e.g., network connections, other files)
	LogFormat       LogFormat              `json:"-"`                // Log format (overrides FormatStr if set)
	ErrorHandler    func(error)            `json:"-"`                // Error handler function for logging errors
}

// Logger is the main logging struct that provides all logging functionality.
// It should be created using NewGourdianLogger() or NewDefaultGourdianLogger().
type Logger struct {
	fileMu          sync.Mutex
	outputsMu       sync.RWMutex
	level           atomic.Int32
	baseFilename    string
	maxBytes        int64
	backupCount     int
	file            *os.File
	multiWriter     io.Writer
	timestampFormat string
	outputs         []io.Writer
	logsDir         string
	closed          atomic.Bool
	rotateChan      chan struct{}
	rotateCloseChan chan struct{}
	wg              sync.WaitGroup
	enableCaller    bool
	asyncQueue      chan *bytes.Buffer
	asyncCloseChan  chan struct{}
	asyncWorkers    int
	format          LogFormat
	fallbackWriter  io.Writer
	errorHandler    func(error)
	rateLimiter     *rate.Limiter
	config          LoggerConfig
	dynamicLevelFn  func() LogLevel
	paused          atomic.Bool
	bufferPool      sync.Pool
}

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel converts a string representation of a log level to a LogLevel constant.
// Valid strings (case-insensitive): "DEBUG", "INFO", "WARN"/"WARNING", "ERROR", "FATAL".
//
// Example:
//
//	level, err := ParseLogLevel("info") // Returns INFO, nil
//	if err != nil {
//	    // handle error
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

// NewGourdianLogger creates a new Logger with the specified configuration.
// The configuration is validated and defaults are applied for missing values.
//
// Example:
//
//	config := LoggerConfig{
//	    Filename: "myapp",
//	    LogLevel: INFO,
//	    MaxBytes: 5 * 1024 * 1024, // 5MB
//	}
//	logger, err := NewGourdianLogger(config)
//	if err != nil {
//	    panic(err)
//	}
func NewGourdianLogger(config LoggerConfig) (*Logger, error) {
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
		config.Filename = "gourdianlogs"
	}
	if config.AsyncWorkers <= 0 && config.BufferSize > 0 {
		config.AsyncWorkers = 1
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
		format:          config.LogFormat,
		errorHandler:    config.ErrorHandler,
		config:          config,
		bufferPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 256))
			},
		},
	}

	if config.EnableFallback {
		logger.fallbackWriter = os.Stderr
	}

	// In NewGourdianLogger(), change the rate limiter initialization to:
	if config.MaxLogRate > 0 {
		// Set burst size to 1 to enforce strict rate limiting
		logger.rateLimiter = rate.NewLimiter(rate.Limit(config.MaxLogRate), 1)
	}

	logger.level.Store(int32(config.LogLevel))
	logger.multiWriter = io.MultiWriter(validOutputs...)

	logger.wg.Add(1)
	go logger.fileSizeRotationWorker()

	if config.BufferSize > 0 {
		logger.asyncQueue = make(chan *bytes.Buffer, config.BufferSize)
		logger.asyncCloseChan = make(chan struct{})
		for i := 0; i < logger.asyncWorkers; i++ {
			logger.wg.Add(1)
			go logger.asyncWorker()
		}
	}

	return logger, nil
}

// NewDefaultGourdianLogger creates a new Logger with default configuration.
// This is the simplest way to create a logger with sensible defaults.
//
// Example:
//
//	logger, err := NewDefaultGourdianLogger()
//	if err != nil {
//	    panic(err)
//	}
//	defer logger.Close()
func NewDefaultGourdianLogger() (*Logger, error) {
	config := LoggerConfig{
		Filename:        defaultFileName,
		MaxBytes:        defaultMaxBytes,
		BackupCount:     defaultBackupCount,
		LogLevel:        defaultLogLevel,
		TimestampFormat: defaultTimestampFormat,
		LogsDir:         defaultLogsDir,
		EnableCaller:    defaultEnableCaller,
		BufferSize:      defaultBufferSize,
		AsyncWorkers:    defaultAsyncWorkers,
		LogFormat:       defaultLogFormat,
		EnableFallback:  defaultEnableFallback,
		MaxLogRate:      defaultMaxLogRate,
	}
	return NewGourdianLogger(config)
}

// Validate checks the LoggerConfig for invalid values.
// Returns an error if any configuration value is invalid.
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

	if lc.FormatStr != "" {
		switch strings.ToUpper(lc.FormatStr) {
		case "PLAIN", "JSON":
		default:
			return fmt.Errorf("invalid log format: %s", lc.FormatStr)
		}
	}
	return nil
}

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

func (l *Logger) asyncWorker() {
	defer l.wg.Done()
	batch := make([]*bytes.Buffer, 0, 100)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case buf := <-l.asyncQueue:
			batch = append(batch, buf)
			if len(batch) >= 100 {
				l.writeBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.writeBatch(batch)
				batch = batch[:0]
			}
		case <-l.asyncCloseChan:
			// Process remaining messages
			if len(batch) > 0 {
				l.writeBatch(batch)
			}
			// Drain the queue
			for {
				select {
				case buf := <-l.asyncQueue:
					l.writeBuffer(buf)
				default:
					return
				}
			}
		}
	}
}

func (l *Logger) writeFallback(message string, args ...interface{}) {
	if l.fallbackWriter == nil {
		return
	}

	var err error
	if len(args) > 0 {
		_, err = fmt.Fprintf(l.fallbackWriter, message, args...)
	} else {
		_, err = fmt.Fprint(l.fallbackWriter, message)
	}

	if err != nil && l.errorHandler != nil {
		l.errorHandler(fmt.Errorf("fallback write error: %w", err))
	}
}

func (l *Logger) writeBatch(buffers []*bytes.Buffer) {

	if l.paused.Load() {
		for _, buf := range buffers {
			l.bufferPool.Put(buf) // Return buffers to pool if paused
		}
		return
	}

	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	for _, buf := range buffers {
		if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
			l.handleError(fmt.Errorf("log write error: %w", err))
			if l.fallbackWriter != nil {
				l.writeFallback("FALLBACK LOG: %s", buf.String())
			}
		}
		l.bufferPool.Put(buf)
	}
}

func (l *Logger) writeBuffer(buf *bytes.Buffer) {

	if l.paused.Load() {
		l.bufferPool.Put(buf) // Return buffer to pool if paused
		return
	}

	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
		l.handleError(fmt.Errorf("log write error: %w", err))
		if l.fallbackWriter != nil {
			l.writeFallback("FALLBACK LOG: %s", buf.String())
		}
	}
	l.bufferPool.Put(buf)
}

func (l *Logger) handleError(err error) {
	if l.errorHandler != nil {
		l.errorHandler(err)
	} else if l.fallbackWriter != nil {
		l.writeFallback("LOGGER ERROR: %v\n", err)
	}
}

func (l *Logger) formatPlain(level LogLevel, message, callerInfo string, fields map[string]interface{}) string {
	var builder strings.Builder
	builder.Grow(256) // Pre-allocate space for common case

	// Write timestamp
	builder.WriteString(time.Now().Format(l.timestampFormat))

	// Write level in brackets
	builder.WriteString(" [")
	builder.WriteString(level.String())
	builder.WriteString("] ")

	// Write caller info if available
	if callerInfo != "" {
		builder.WriteString(callerInfo)
		builder.WriteString(": ")
	}

	// Write message
	builder.WriteString(message)

	// Write fields if available
	if len(fields) > 0 {
		builder.WriteString(" {")
		first := true
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys) // Sort fields for consistent output

		for _, k := range keys {
			if !first {
				builder.WriteString(", ")
			}
			first = false
			builder.WriteString(k)
			builder.WriteString("=")
			fmt.Fprintf(&builder, "%v", fields[k])
		}
		builder.WriteString("}")
	}

	builder.WriteString("\n")
	return builder.String()
}

func (l *Logger) formatJSON(level LogLevel, message, callerInfo string, fields map[string]interface{}) string {
	logEntry := make(map[string]interface{}, len(fields)+4)
	logEntry["timestamp"] = time.Now().Format(l.timestampFormat)
	logEntry["level"] = level.String()
	logEntry["message"] = message

	if l.enableCaller && callerInfo != "" {
		logEntry["caller"] = callerInfo
	}

	for k, v := range l.config.CustomFields {
		logEntry[k] = v
	}

	for k, v := range fields {
		logEntry[k] = v
	}

	var jsonData []byte
	var err error

	if l.config.PrettyPrint {
		jsonData, err = json.MarshalIndent(logEntry, "", "  ")
	} else {
		jsonData, err = json.Marshal(logEntry)
	}

	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal log entry: %v"}`+"\n", err)
	}
	return string(jsonData) + "\n"
}

func (l *Logger) getCallerInfo() string {
	if !l.enableCaller {
		return ""
	}

	pc, file, line, ok := runtime.Caller(3) // Adjusted depth for logger methods
	if !ok {
		return ""
	}

	// Get function name
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	// Extract just the function name without package path
	fnName := fn.Name()
	if lastSlash := strings.LastIndex(fnName, "/"); lastSlash >= 0 {
		fnName = fnName[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(fnName, "."); lastDot >= 0 {
		fnName = fnName[lastDot+1:]
	}

	return fmt.Sprintf("%s:%d:%s", filepath.Base(file), line, fnName)
}

func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if l.paused.Load() {
		return
	}

	// Check rate limiting before doing any work
	if l.rateLimiter != nil {
		if !l.rateLimiter.Allow() {
			return // Silently drop the message if rate limited
		}
	}

	// Rest of the existing log method...
	var currentLevel LogLevel
	if l.dynamicLevelFn != nil {
		currentLevel = l.dynamicLevelFn()
	} else {
		currentLevel = LogLevel(l.level.Load())
	}

	if level < currentLevel {
		return
	}

	var callerInfo string
	if l.enableCaller {
		callerInfo = l.getCallerInfo()
	}

	var formatted string
	switch l.format {
	case FormatJSON:
		formatted = l.formatJSON(level, message, callerInfo, fields)
	default:
		formatted = l.formatPlain(level, message, callerInfo, fields)
	}

	if l.closed.Load() {
		l.writeFallback("Logger closed. Message: %s", formatted)
		return
	}

	if l.asyncQueue != nil {
		buf := l.bufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		buf.WriteString(formatted)

		select {
		case l.asyncQueue <- buf:
		default:
			l.writeBuffer(buf)
		}
	} else {
		l.fileMu.Lock()
		defer l.fileMu.Unlock()
		if l.closed.Load() {
			l.writeFallback("Logger closed. Message: %s", formatted)
			return
		}
		if _, err := l.multiWriter.Write([]byte(formatted)); err != nil {
			l.handleError(fmt.Errorf("log write error: %w", err))
			l.writeFallback("FALLBACK LOG: %s", formatted)
		}
	}

	if level == FATAL {
		l.Flush()
		os.Exit(1)
	}
}

func (l *Logger) rotateLogFiles() error {
	if l.file == nil {
		return fmt.Errorf("log file not open")
	}

	fileInfo, err := l.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.Size() < l.maxBytes {
		return nil
	}

	// Close current file
	oldFile := l.file
	if err := oldFile.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	// Create new filename with timestamp
	base := strings.TrimSuffix(l.baseFilename, ".log")
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_%s.log", base, timestamp)

	// Rename current file to backup
	renameErr := os.Rename(l.baseFilename, backupPath)

	// Try to reopen the file regardless of whether rename succeeded
	file, openErr := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if openErr != nil {
		// If we can't reopen the file, try to reopen the original file we just closed
		if renameErr == nil {
			if recoveredFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				l.file = recoveredFile
				return fmt.Errorf("failed to create new log file (%v), recovered original: %w", openErr, renameErr)
			}
		}
		return fmt.Errorf("failed to create new log file and couldn't recover original: %w", openErr)
	}

	// Update logger state
	l.file = file

	// Update multiWriter with new file
	l.outputsMu.Lock()
	outputs := []io.Writer{file}
	if len(l.outputs) > 1 { // Keep other outputs
		outputs = append(outputs, l.outputs[1:]...)
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)
	l.outputsMu.Unlock()

	if renameErr != nil {
		return fmt.Errorf("failed to rename log file but continued logging: %w", renameErr)
	}

	// Clean up old backups
	l.cleanupOldBackups()
	return nil
}

func (l *Logger) cleanupOldBackups() {
	pattern := strings.TrimSuffix(l.baseFilename, ".log") + "_*.log"
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) <= l.backupCount {
		return
	}

	sort.Slice(files, func(i, j int) bool {
		info1, _ := os.Stat(files[i])
		info2, _ := os.Stat(files[j])
		return info1.ModTime().Before(info2.ModTime())
	})

	for _, f := range files[:len(files)-l.backupCount] {
		_ = os.Remove(f)
	}
}

// Flush ensures all buffered log messages are written to their destinations.
// This is particularly important for asynchronous logging to ensure messages
// are written before program exit.
//
// Example:
//
//	logger.Info("Important message")
//	logger.Flush() // Ensure message is written before proceeding
func (l *Logger) Flush() {
	if l.asyncQueue != nil {
		// Wait for the queue to be empty
		for len(l.asyncQueue) > 0 {
			time.Sleep(10 * time.Millisecond)
		}

		// Give async workers a moment to process any in-flight messages
		time.Sleep(100 * time.Millisecond)
	}

	// If there are any async workers, wait for them to finish current work
	if l.asyncWorkers > 0 {
		// This gives time for any batch writes to complete
		time.Sleep(100 * time.Millisecond)
	}
}

// SetLogLevel changes the minimum log level that will be processed.
// Messages below this level will be ignored.
//
// Example:
//
//	logger.SetLogLevel(gourdianlogger.WARN) // Only log WARN and above
func (l *Logger) SetLogLevel(level LogLevel) {
	l.level.Store(int32(level))
}

// GetLogLevel returns the current minimum log level.
//
// Example:
//
//	if logger.GetLogLevel() <= gourdianlogger.DEBUG {
//	    logger.Debug("Debug message")
//	}
func (l *Logger) GetLogLevel() LogLevel {
	return LogLevel(l.level.Load())
}

// SetDynamicLevelFunc sets a function that will be called for each log message
// to determine the current log level. This allows for runtime log level changes
// without requiring explicit synchronization.
//
// Example:
//
//	logger.SetDynamicLevelFunc(func() LogLevel {
//	    if config.DebugMode {
//	        return DEBUG
//	    }
//	    return INFO
//	})
func (l *Logger) SetDynamicLevelFunc(fn func() LogLevel) {
	l.dynamicLevelFn = fn
}

// AddOutput adds an additional io.Writer destination for log messages.
// Useful for sending logs to multiple destinations (e.g., file and network).
//
// Example:
//
//	file, _ := os.Create("additional.log")
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

// RemoveOutput removes a previously added output writer.
//
// Example:
//
//	logger.RemoveOutput(os.Stdout) // Stop logging to stdout
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

// Close cleanly shuts down the logger, flushing any buffered messages
// and closing open files. Should be called before program exit.
//
// Example:
//
//	defer logger.Close()
func (l *Logger) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}

	// First stop accepting new messages
	if l.asyncCloseChan != nil {
		close(l.asyncCloseChan)
	}
	close(l.rotateCloseChan)

	// Wait for all workers to finish processing
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Timeout reached, proceed with closing anyway
	}

	// Now safely close the file
	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// IsClosed returns true if the logger has been closed.
//
// Example:
//
//	if !logger.IsClosed() {
//	    logger.Info("Still logging")
//	}
func (l *Logger) IsClosed() bool {
	return l.closed.Load()
}

// Pause temporarily stops processing log messages.
// Useful for reducing logging overhead during critical sections.
//
// Example:
//
//	logger.Pause()
//	defer logger.Resume()
//	// Critical section with minimal overhead
func (l *Logger) Pause() {
	l.paused.Store(true)
}

// Resume resumes processing of log messages after a Pause().
func (l *Logger) Resume() {
	l.paused.Store(false)
}

// IsPaused returns true if the logger is currently paused.
func (l *Logger) IsPaused() bool {
	return l.paused.Load()
}

// Debug logs a message at DEBUG level.
// Arguments are handled like fmt.Sprint().
//
// Example:
//
//	logger.Debug("Current value:", value)
func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), nil)
}

// Info logs a message at INFO level.
// Arguments are handled like fmt.Sprint().
func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), nil)
}

// Warn logs a message at WARN level.
// Arguments are handled like fmt.Sprint().
func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), nil)
}

// Error logs a message at ERROR level.
// Arguments are handled like fmt.Sprint().
func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), nil)
}

// Fatal logs a message at FATAL level and exits the program with status 1.
// Arguments are handled like fmt.Sprint().
func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), nil)
	// l.Flush()
	// os.Exit(1)
}

// Debugf logs a formatted message at DEBUG level.
// Arguments are handled like fmt.Sprintf().
//
// Example:
//
//	logger.Debugf("User %s logged in from %s", user, ip)
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), nil)
}

// Infof logs a formatted message at INFO level.
// Arguments are handled like fmt.Sprintf().
func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), nil)
}

// Warnf logs a formatted message at WARN level.
// Arguments are handled like fmt.Sprintf().
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), nil)
}

// Errorf logs a formatted message at ERROR level.
// Arguments are handled like fmt.Sprintf().
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), nil)
}

// Fatalf logs a formatted message at FATAL level and exits the program with status 1.
// Arguments are handled like fmt.Sprintf().
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), nil)
	// l.Flush()
	// os.Exit(1)
}

// DebugWithFields logs a message at DEBUG level with additional structured fields.
// Fields will be included in JSON output or as key-value pairs in text output.
//
// Example:
//
//	logger.DebugWithFields(map[string]interface{}{
//	    "user": username,
//	    "attempt": 3,
//	}, "Login attempt")
func (l *Logger) DebugWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), fields)
}

// InfoWithFields logs a message at INFO level with additional structured fields.
func (l *Logger) InfoWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), fields)
}

// WarnWithFields logs a message at WARN level with additional structured fields.
func (l *Logger) WarnWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), fields)
}

// ErrorWithFields logs a message at ERROR level with additional structured fields.
func (l *Logger) ErrorWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), fields)
}

// FatalWithFields logs a message at FATAL level with additional structured fields
// and exits the program with status 1.
func (l *Logger) FatalWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), fields)
	l.Flush()
	os.Exit(1)
}

// DebugfWithFields logs a formatted message at DEBUG level with additional structured fields.
//
// Example:
//
//	logger.DebugfWithFields(map[string]interface{}{
//	    "duration": dur,
//	}, "Process took %.2f seconds", dur.Seconds())
func (l *Logger) DebugfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), fields)
}

// InfofWithFields logs a formatted message at INFO level with additional structured fields.
func (l *Logger) InfofWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), fields)
}

// WarnfWithFields logs a formatted message at WARN level with additional structured fields.
func (l *Logger) WarnfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), fields)
}

// ErrorfWithFields logs a formatted message at ERROR level with additional structured fields.
func (l *Logger) ErrorfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), fields)
}

// FatalfWithFields logs a formatted message at FATAL level with additional structured fields
// and exits the program with status 1.
func (l *Logger) FatalfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), fields)
	l.Flush()
	os.Exit(1)
}
