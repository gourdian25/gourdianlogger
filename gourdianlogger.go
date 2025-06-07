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

type LogLevel int32

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

type LogFormat int

const (
	FormatPlain LogFormat = iota
	FormatJSON
)

var (
	defaultBackupCount     int       = 5
	defaultBufferSize      int       = 0
	defaultAsyncWorkers    int       = 1
	defaultMaxLogRate      int       = 0
	defaultEnableCaller    bool      = true
	defaultEnableFallback  bool      = true
	defaultLogsDir         string    = "logs"
	defaultLogLevel        LogLevel  = DEBUG
	defaultLogFormat       LogFormat = FormatPlain
	defaultFileName        string    = "gourdianlogs"
	defaultMaxBytes        int64     = 10 * 1024 * 1024
	defaultTimestampFormat string    = "2006-01-02 15:04:05.000000"
)

type LoggerConfig struct {
	BackupCount     int                    `json:"backup_count"`     // Number of backups to keep
	BufferSize      int                    `json:"buffer_size"`      // Buffer size for async logging
	AsyncWorkers    int                    `json:"async_workers"`    // Number of async workers
	MaxLogRate      int                    `json:"max_log_rate"`     // Max logs per second (0 for unlimited)
	EnableCaller    bool                   `json:"enable_caller"`    // Include caller info
	EnableFallback  bool                   `json:"enable_fallback"`  // Whether to use fallback logging
	PrettyPrint     bool                   `json:"pretty_print"`     // For human-readable JSON
	MaxBytes        int64                  `json:"max_bytes"`        // Max file size before rotation
	Filename        string                 `json:"filename"`         // Base filename for logs
	TimestampFormat string                 `json:"timestamp_format"` // Custom timestamp format
	LogsDir         string                 `json:"logs_dir"`         // Directory for log files
	FormatStr       string                 `json:"format"`           // Format as string (for config)
	LogLevel        LogLevel               `json:"log_level"`        // Log level as string (for config)
	CustomFields    map[string]interface{} `json:"custom_fields"`    // Custom fields to include
	Outputs         []io.Writer            `json:"-"`                // Additional outputs
	LogFormat       LogFormat              `json:"-"`                // Log message format (internal use)
	ErrorHandler    func(error)            `json:"-"`                // Custom error handler
}

type Logger struct {
	fileMu          sync.Mutex
	bufferPoolMu    sync.Mutex
	outputsMu       sync.RWMutex
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
	fallbackWriter  io.Writer
	errorHandler    func(error)
	rateLimiter     *rate.Limiter
	config          LoggerConfig
	dynamicLevelFn  func() LogLevel
	paused          atomic.Bool
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

func DefaultConfig() LoggerConfig {
	return LoggerConfig{
		Filename:        defaultFileName,
		MaxBytes:        defaultMaxBytes,
		BackupCount:     defaultBackupCount,
		LogLevel:        defaultLogLevel,
		TimestampFormat: defaultTimestampFormat,
		LogsDir:         defaultLogsDir,
		EnableCaller:    defaultEnableCaller,
		BufferSize:      defaultBufferSize, // Sync by default
		AsyncWorkers:    defaultAsyncWorkers,
		LogFormat:       defaultLogFormat,
		EnableFallback:  defaultEnableFallback,
		MaxLogRate:      defaultMaxLogRate, // Unlimited by default
	}
}

func NewGourdianLogger(config LoggerConfig) (*Logger, error) {
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

func NewGourdianLoggerWithDefault() (*Logger, error) {
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

	// Add custom fields from config
	for k, v := range l.config.CustomFields {
		logEntry[k] = v
	}

	// Add log-specific fields
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

// getCallerInfo retrieves information about the caller
func (l *Logger) getCallerInfo() string {
	if !l.enableCaller {
		return ""
	}

	// Skip 3 frames to get the actual caller:
	// 0 - runtime.Caller
	// 1 - this function
	// 2 - formatMessage
	// 3 - the actual logging function (Debug, Info, etc.)
	pc, file, line, ok := runtime.Caller(3)
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
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	// Check if logger is paused
	if l.paused.Load() {
		return
	}

	// Apply rate limiting if configured
	if l.rateLimiter != nil && !l.rateLimiter.Allow() {
		return
	}

	var callerInfo string
	if l.enableCaller {
		callerInfo = l.getCallerInfo()
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
	if l.file == nil {
		return fmt.Errorf("log file not open")
	}

	// Get current file info
	fileInfo, err := l.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Only rotate if size threshold is met
	if fileInfo.Size() < l.maxBytes {
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

	l.cleanupOldBackups()

	return nil
}

// cleanupOldBackups removes old log backups exceeding the backup count
func (l *Logger) cleanupOldBackups() {
	pattern := strings.TrimSuffix(l.baseFilename, ".log") + "_*.log"
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

func (l *Logger) SetLogLevel(level LogLevel) {
	l.level.Store(int32(level))
}

func (l *Logger) GetLogLevel() LogLevel {
	return LogLevel(l.level.Load())
}

func (l *Logger) SetDynamicLevelFunc(fn func() LogLevel) {
	l.dynamicLevelFn = fn
}

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

func (l *Logger) Pause() {
	l.paused.Store(true)
}

func (l *Logger) Resume() {
	l.paused.Store(false)
}

func (l *Logger) IsPaused() bool {
	return l.paused.Load()
}

func (l *Logger) WithPause(fn func()) {
	l.Pause()
	defer l.Resume()
	fn()
}

func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), nil)
}

func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), nil)
}

func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), nil)
}

func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), nil)
}

func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), nil)
	l.Flush()
	os.Exit(1)
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), nil)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), nil)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), nil)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), nil)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), nil)
	l.Flush()
	os.Exit(1)
}

func (l *Logger) DebugWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), fields)
}

func (l *Logger) InfoWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), fields)
}

func (l *Logger) WarnWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), fields)
}

func (l *Logger) ErrorWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), fields)
}

func (l *Logger) FatalWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), fields)
	l.Flush()
	os.Exit(1)
}

func (l *Logger) DebugfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), fields)
}

func (l *Logger) InfofWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), fields)
}

func (l *Logger) WarnfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), fields)
}

func (l *Logger) ErrorfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), fields)
}

func (l *Logger) FatalfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), fields)
	l.Flush()
	os.Exit(1)
}

func (l *Logger) Flush() {
	if l.asyncQueue != nil {
		for len(l.asyncQueue) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (l *Logger) IsClosed() bool {
	return l.closed.Load()
}

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
		lc.LogFormat = FormatPlain
	} else {
		switch strings.ToUpper(aux.FormatStr) {
		case "PLAIN":
			lc.LogFormat = FormatPlain
		case "JSON":
			lc.LogFormat = FormatJSON
		default:
			return fmt.Errorf("invalid log format: %s", aux.FormatStr)
		}
	}

	return nil
}
