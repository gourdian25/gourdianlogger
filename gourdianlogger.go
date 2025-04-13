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
type LoggerConfig struct {
	Filename        string       `json:"filename"`         // Base filename for logs
	MaxBytes        int64        `json:"max_bytes"`        // Max file size before rotation
	BackupCount     int          `json:"backup_count"`     // Number of backups to keep
	LogLevel        LogLevel     `json:"log_level"`        // Minimum log level
	TimestampFormat string       `json:"timestamp_format"` // Custom timestamp format
	Outputs         []io.Writer  `json:"-"`                // Additional outputs
	LogsDir         string       `json:"logs_dir"`         // Directory for log files
	EnableCaller    bool         `json:"enable_caller"`    // Include caller info
	BufferSize      int          `json:"buffer_size"`      // Buffer size for async logging
	AsyncWorkers    int          `json:"async_workers"`    // Number of async workers
	Format          LogFormat    `json:"format"`           // Log message format
	FormatConfig    FormatConfig `json:"format_config"`    // Format-specific config
}

// Simplified FormatConfig
type FormatConfig struct {
	PrettyPrint  bool                   `json:"pretty_print"`  // For human-readable JSON
	CustomFields map[string]interface{} `json:"custom_fields"` // Custom fields to include
}

// Logger is the main logging struct
type Logger struct {
	mu               sync.RWMutex
	level            atomic.Int32
	baseFilename     string
	maxBytes         int64
	backupCount      int
	file             *os.File
	multiWriter      io.Writer
	bufferPool       sync.Pool
	timestampFormat  string
	outputs          []io.Writer
	logsDir          string
	closed           atomic.Bool
	rotateChan       chan struct{}
	rotateCloseChan  chan struct{}
	wg               sync.WaitGroup
	enableCaller     bool
	asyncQueue       chan *logEntry
	asyncCloseChan   chan struct{}
	asyncWorkerCount int
	format           LogFormat
	formatConfig     FormatConfig
}

type logEntry struct {
	level      LogLevel
	message    string
	callerInfo string
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
	}
}

func NewGourdianLogger(config LoggerConfig) (*Logger, error) {
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
		baseFilename:     logPath,
		maxBytes:         config.MaxBytes,
		backupCount:      config.BackupCount,
		file:             file,
		timestampFormat:  config.TimestampFormat,
		outputs:          validOutputs,
		logsDir:          config.LogsDir,
		rotateChan:       make(chan struct{}, 1),
		rotateCloseChan:  make(chan struct{}),
		enableCaller:     config.EnableCaller,
		asyncWorkerCount: config.AsyncWorkers,
		format:           config.Format,
		formatConfig:     config.FormatConfig,
	}

	logger.level.Store(int32(config.LogLevel))
	logger.multiWriter = io.MultiWriter(validOutputs...)
	logger.bufferPool.New = func() interface{} {
		return new(bytes.Buffer)
	}

	logger.wg.Add(1)
	go logger.rotationWorker()

	if config.BufferSize > 0 {
		logger.asyncQueue = make(chan *logEntry, config.BufferSize)
		logger.asyncCloseChan = make(chan struct{})

		for i := 0; i < logger.asyncWorkerCount; i++ {
			logger.wg.Add(1)
			go logger.asyncWorker()
		}
	}

	return logger, nil
}

// NewGourdianLoggerWithDefault initializes a logger with DefaultConfig values.
// It returns a ready-to-use *Logger instance or an error if setup fails.
func NewGourdianLoggerWithDefault() (*Logger, error) {
	config := DefaultConfig()
	return NewGourdianLogger(config)
}

func (l *Logger) rotationWorker() {
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

func (l *Logger) asyncWorker() {
	defer l.wg.Done()

	for {
		select {
		case entry := <-l.asyncQueue:
			l.processLogEntry(entry.level, entry.message, entry.callerInfo)
		case <-l.asyncCloseChan:
			for {
				select {
				case entry := <-l.asyncQueue:
					l.processLogEntry(entry.level, entry.message, entry.callerInfo)
				default:
					return
				}
			}
		}
	}
}

func (l *Logger) processLogEntry(level LogLevel, message string, callerInfo string) {
	if level < l.GetLogLevel() {
		return
	}

	buf := l.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer l.bufferPool.Put(buf)

	buf.WriteString(l.formatMessage(level, message, callerInfo))

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed.Load() {
		fmt.Fprintf(os.Stderr, "Logger closed. Message: %s", buf.String())
		return
	}

	if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "Log write error: %v\n", err)
	}

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) formatPlain(level LogLevel, message string, callerInfo string) string {
	levelStr := fmt.Sprintf("[%s]", level.String())
	timestampStr := time.Now().Format(l.timestampFormat)

	var callerPart string
	if callerInfo != "" {
		callerPart = callerInfo + ":"
	}

	return fmt.Sprintf("%s %s %s%s\n",
		timestampStr,
		levelStr,
		callerPart,
		message,
	)
}

func (l *Logger) formatJSON(level LogLevel, message string, callerInfo string) string {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(l.timestampFormat),
		"level":     level.String(),
		"message":   message,
	}

	if l.enableCaller && callerInfo != "" {
		logEntry["caller"] = callerInfo
	}

	for k, v := range l.formatConfig.CustomFields {
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
		return fmt.Sprintf(`{"error":"failed to marshal log entry: %v"}`, err)
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

func (l *Logger) formatMessage(level LogLevel, message string, callerInfo string) string {
	switch l.format {
	case FormatJSON:
		return l.formatJSON(level, message, callerInfo)
	default:
		return l.formatPlain(level, message, callerInfo)
	}
}

func (l *Logger) log(level LogLevel, message string, skip int) {
	var callerInfo string
	if l.enableCaller {
		callerInfo = l.getCallerInfo(skip)
	}

	if l.asyncQueue != nil {
		select {
		case l.asyncQueue <- &logEntry{level, message, callerInfo}:
		default:
			l.processLogEntry(level, message, callerInfo)
		}
	} else {
		l.processLogEntry(level, message, callerInfo)
	}
}

func (l *Logger) rotateLogFiles() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	base := strings.TrimSuffix(l.baseFilename, ".log")
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_%s.log", base, timestamp)

	if err := os.Rename(l.baseFilename, backupPath); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	file, err := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	l.file = file
	outputs := []io.Writer{os.Stdout, file}
	if len(l.outputs) > 2 {
		outputs = append(outputs, l.outputs[2:]...)
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)

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
		return files[i] < files[j] // oldest first
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

	close(l.rotateCloseChan)
	if l.asyncCloseChan != nil {
		close(l.asyncCloseChan)
	}
	l.wg.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...), 3)
}

func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...), 3)
}

func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...), 3)
}

func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...), 3)
}

func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...), 3)
	l.Flush()
	os.Exit(1)
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...), 3)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...), 3)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...), 3)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...), 3)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...), 3)
	l.Flush()
	os.Exit(1)
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

	// If the input is just an empty object, use default values
	if string(data) == "{}" {
		*lc = DefaultConfig()
		return nil
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Set default log level if not specified
	if aux.LogLevelStr == "" {
		lc.LogLevel = DEBUG
	} else {
		level, err := ParseLogLevel(aux.LogLevelStr)
		if err != nil {
			return err
		}
		lc.LogLevel = level
	}

	// Set default format if not specified
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

	return nil
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
