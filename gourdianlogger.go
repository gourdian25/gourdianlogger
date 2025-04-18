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

// LogLevel represents the severity level of log messages
type LogLevel int32

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// LogFormat represents the format of log messages
type LogFormat int

const (
	FormatPlain LogFormat = iota // Default plain text format
	FormatJSON                   // JSON format
)

var (
	defaultMaxBytes        int64  = 10 * 1024 * 1024 // 10MB
	defaultBackupCount     int    = 5
	defaultTimestampFormat string = "2006-01-02 15:04:05.000000"
	defaultLogsDir         string = "logs"
)

// LoggerConfig holds configuration for the logger
// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	Filename        string        `json:"filename"`         // Base filename for logs
	MaxBytes        int64         `json:"max_bytes"`        // Max file size before rotation
	BackupCount     int           `json:"backup_count"`     // Number of backups to keep
	LogLevel        LogLevel      `json:"-"`                // Minimum log level (internal use)
	LogLevelStr     string        `json:"log_level"`        // Log level as string (for config)
	TimestampFormat string        `json:"timestamp_format"` // Custom timestamp format
	Outputs         []io.Writer   `json:"-"`                // Additional outputs
	LogsDir         string        `json:"logs_dir"`         // Directory for log files
	EnableCaller    bool          `json:"enable_caller"`    // Include caller info
	BufferSize      int           `json:"buffer_size"`      // Buffer size for async logging
	AsyncWorkers    int           `json:"async_workers"`    // Number of async workers
	Format          LogFormat     `json:"-"`                // Log message format (internal use)
	FormatStr       string        `json:"format"`           // Format as string (for config)
	FormatConfig    FormatConfig  `json:"format_config"`    // Format-specific config
	EnableFallback  bool          `json:"enable_fallback"`  // Whether to use fallback logging
	ErrorHandler    func(error)   `json:"-"`                // Custom error handler
	MaxLogRate      int           `json:"max_log_rate"`     // Max logs per second (0 for unlimited)
	CompressBackups bool          `json:"compress_backups"` // Whether to gzip rotated logs
	RotationTime    time.Duration `json:"rotation_time"`    // Time-based rotation interval
	SampleRate      int           `json:"sample_rate"`      // Log sampling rate (1 in N)
	CallerDepth     int           `json:"caller_depth"`     // How many stack frames to skip
}

// FormatConfig contains format-specific configuration
type FormatConfig struct {
	PrettyPrint  bool                   `json:"pretty_print"`  // For human-readable JSON
	CustomFields map[string]interface{} `json:"custom_fields"` // Custom fields to include
}

// Logger is the main logging struct
type Logger struct {
	mu              sync.RWMutex
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

// Complete initializes the internal fields from string representations
func (lc *LoggerConfig) Complete() error {
	// Parse log level
	if lc.LogLevelStr != "" {
		level, err := ParseLogLevel(lc.LogLevelStr)
		if err != nil {
			return fmt.Errorf("invalid log level: %w", err)
		}
		lc.LogLevel = level
	} else {
		lc.LogLevel = DEBUG // default
	}

	// Parse format
	if lc.FormatStr != "" {
		switch strings.ToUpper(lc.FormatStr) {
		case "PLAIN":
			lc.Format = FormatPlain
		case "JSON":
			lc.Format = FormatJSON
		default:
			return fmt.Errorf("invalid log format: %s", lc.FormatStr)
		}
	} else {
		lc.Format = FormatPlain // default
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

// NewGourdianLoggerWithDefault initializes a logger with DefaultConfig values
func NewGourdianLoggerWithDefault() (*Logger, error) {
	config := DefaultConfig()
	return NewGourdianLogger(config)
}

// fileSizeRotationWorker listens for manual rotation triggers (e.g., after write or on demand)
func (l *Logger) fileSizeRotationWorker() {
	defer l.wg.Done()
	for {
		select {
		case <-l.rotateChan:
			l.mu.Lock()
			if l.file != nil {
				if err := l.rotateLogFiles(); err != nil {
					l.handleError(fmt.Errorf("log rotation failed: %w", err))
				}
			}
			l.mu.Unlock()
		case <-l.rotateCloseChan:
			return
		}
	}
}

// timeRotationWorker triggers rotation at regular intervals if RotationTime is set
func (l *Logger) timeRotationWorker(interval time.Duration) {
	defer l.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			if l.file != nil {
				if err := l.rotateLogFiles(); err != nil {
					l.handleError(fmt.Errorf("time-based log rotation failed: %w", err))
				}
			}
			l.mu.Unlock()
		case <-l.rotateCloseChan:
			return
		}
	}
}

func (l *Logger) asyncWorker() {
	defer l.wg.Done()

	for {
		select {
		case entry := <-l.asyncQueue:
			l.processLogEntry(entry.level, entry.message, entry.callerInfo, entry.fields)
		case <-l.asyncCloseChan:
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

func (l *Logger) handleError(err error) {
	if l.errorHandler != nil {
		l.errorHandler(err)
	} else if l.fallbackWriter != nil {
		fmt.Fprintf(l.fallbackWriter, "LOGGER ERROR: %v\n", err)
	}
}

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

	buf := l.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer l.bufferPool.Put(buf)

	buf.WriteString(l.formatMessage(level, message, callerInfo, fields))

	l.mu.Lock()
	defer l.mu.Unlock()

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

func (l *Logger) formatMessage(level LogLevel, message string, callerInfo string, fields map[string]interface{}) string {
	switch l.format {
	case FormatJSON:
		return l.formatJSON(level, message, callerInfo, fields)
	default:
		return l.formatPlain(level, message, callerInfo, fields)
	}
}

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

func (l *Logger) rotateLogFiles() error {
	t := time.Now()
	defer func() {
		l.Debugf("rotateLogFiles completed in %v", time.Since(t))
	}()

	if l.file == nil {
		return fmt.Errorf("log file not open")
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
	outputs := []io.Writer{os.Stdout, file}
	if len(l.outputs) > 2 {
		outputs = append(outputs, l.outputs[2:]...)
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)

	// Compress backup if configured
	if l.config.CompressBackups {
		go func() {
			if err := compressFile(backupPath); err != nil {
				l.handleError(fmt.Errorf("failed to compress log file: %w", err))
			}
		}()
	}

	l.cleanupOldBackups()
	return nil
}

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

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed.Load() {
		return
	}

	l.outputs = append(l.outputs, output)
	l.multiWriter = io.MultiWriter(l.outputs...)
}

func (l *Logger) RemoveOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

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

	l.mu.Lock()
	defer l.mu.Unlock()

	// Close file last
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Basic log methods
func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), l.config.CallerDepth, nil)
}

func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), l.config.CallerDepth, nil)
	l.Flush()
	os.Exit(1)
}

// Formatted log methods
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), l.config.CallerDepth, nil)
	l.Flush()
	os.Exit(1)
}

// Structured logging methods
func (l *Logger) DebugWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

func (l *Logger) InfoWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

func (l *Logger) WarnWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

func (l *Logger) ErrorWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), l.config.CallerDepth, fields)
}

func (l *Logger) FatalWithFields(fields map[string]interface{}, v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), l.config.CallerDepth, fields)
	l.Flush()
	os.Exit(1)
}

func (l *Logger) DebugfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

func (l *Logger) InfofWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

func (l *Logger) WarnfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

func (l *Logger) ErrorfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
}

func (l *Logger) FatalfWithFields(fields map[string]interface{}, format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), l.config.CallerDepth, fields)
	l.Flush()
	os.Exit(1)
}

// Flush ensures all buffered logs are written
func (l *Logger) Flush() {
	if l.asyncQueue != nil {
		for len(l.asyncQueue) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// IsClosed returns whether the logger has been closed
func (l *Logger) IsClosed() bool {
	return l.closed.Load()
}

// WithConfig creates a logger from JSON configuration
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
