package gourdianlogger

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
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
	FormatPlain  LogFormat = iota // Default plain text format
	FormatJSON                    // JSON format
	FormatCLF                     // Common Log Format (for HTTP)
	FormatGELF                    // Graylog Extended Log Format
	FormatLogfmt                  // Logfmt key=value format
	FormatCSV                     // Comma-separated values
	FormatXML                     // XML format
	FormatCEF                     // Common Event Format (for security)
)

var (
	// Default values
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
	Outputs         []io.Writer  `json:"-"`                // Additional outputs (not JSON serializable)
	LogsDir         string       `json:"logs_dir"`         // Directory for log files
	EnableCaller    bool         `json:"enable_caller"`    // Include caller info
	BufferSize      int          `json:"buffer_size"`      // Buffer size for async logging
	AsyncWorkers    int          `json:"async_workers"`    // Number of async workers
	Format          LogFormat    `json:"format"`           // Log message format
	FormatConfig    FormatConfig `json:"format_config"`    // Format-specific config
}

// FormatConfig holds format-specific configuration
type FormatConfig struct {
	// JSON/XML specific
	PrettyPrint bool `json:"pretty_print"` // For human-readable JSON/XML

	// CSV specific
	CSVHeaders   bool `json:"csv_headers"`   // Include headers in CSV
	CSVDelimiter rune `json:"csv_delimiter"` // Custom delimiter

	// Field customization
	TimestampField string `json:"timestamp_field"` // Custom field name
	LevelField     string `json:"level_field"`
	MessageField   string `json:"message_field"`
	CallerField    string `json:"caller_field"`

	// Custom fields to include in all formats
	CustomFields map[string]interface{} `json:"custom_fields"`
}

// Logger is the main logging struct
type Logger struct {
	mu                sync.RWMutex   // Protects non-atomic operations
	level             atomic.Int32   // Atomic log level
	baseFilename      string         // Base log file path
	maxBytes          int64          // Max file size
	backupCount       int            // Number of backups
	file              *os.File       // Current log file
	multiWriter       io.Writer      // Combined output
	bufferPool        sync.Pool      // Reusable buffers
	timestampFormat   string         // Time format
	outputs           []io.Writer    // All outputs
	logsDir           string         // Log directory
	closed            atomic.Bool    // Atomic closed flag
	rotateChan        chan struct{}  // Rotation signal
	rotateCloseChan   chan struct{}  // Rotation stop signal
	wg                sync.WaitGroup // Background ops
	enableCaller      bool           // Include caller info
	asyncQueue        chan *logEntry // Async logging queue
	asyncCloseChan    chan struct{}  // Async stop signal
	asyncWorkerCount  int            // Number of async workers
	format            LogFormat      // Log message format
	formatConfig      FormatConfig
	csvHeadersWritten bool // For CSV header tracking
}

type logEntry struct {
	level   LogLevel
	message string
}

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

// DefaultConfig returns a LoggerConfig with default values
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
		AsyncWorkers:    1, // Default workers if async enabled
		Format:          FormatPlain,
		// In DefaultConfig():
		FormatConfig: FormatConfig{
			TimestampField: "timestamp",
			LevelField:     "level",
			MessageField:   "message",
			CallerField:    "caller",
			CSVDelimiter:   ',',
		},
	}
}

// NewGourdianLogger creates a new configured logger instance
func NewGourdianLogger(config LoggerConfig) (*Logger, error) {
	// Apply defaults with validation
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

	// Clean filename and ensure directory exists
	config.Filename = strings.TrimSpace(strings.TrimSuffix(config.Filename, ".log"))
	if err := os.MkdirAll(config.LogsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// In NewGourdianLogger, after directory creation:
	absLogsDir, err := filepath.Abs(config.LogsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for logs directory: %w", err)
	}
	fmt.Printf("Logs will be written to: %s\n", absLogsDir) // Debug output

	logPath := filepath.Join(config.LogsDir, config.Filename+".log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Setup outputs with stdout and file as defaults
	outputs := []io.Writer{file, os.Stdout} // File and stdout
	if config.Outputs != nil {
		outputs = append(outputs, config.Outputs...)
	}

	// Filter out nil writers
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
	}

	logger.level.Store(int32(config.LogLevel))
	logger.multiWriter = io.MultiWriter(validOutputs...)
	logger.bufferPool.New = func() interface{} {
		return new(bytes.Buffer)
	}

	// Start background workers
	logger.wg.Add(1)
	go logger.rotationWorker()

	// Setup async logging if enabled
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

// rotationWorker handles log file rotation in the background
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

// asyncWorker processes log entries asynchronously
func (l *Logger) asyncWorker() {
	defer l.wg.Done()

	for {
		select {
		case entry := <-l.asyncQueue:
			l.processLogEntry(entry.level, entry.message)
		case <-l.asyncCloseChan:
			// Drain remaining messages
			for {
				select {
				case entry := <-l.asyncQueue:
					l.processLogEntry(entry.level, entry.message)
				default:
					return
				}
			}
		}
	}
}

// processLogEntry handles the core logging logic
func (l *Logger) processLogEntry(level LogLevel, message string) {
	if level < l.GetLogLevel() {
		return
	}

	buf := l.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer l.bufferPool.Put(buf)

	buf.WriteString(l.formatMessage(level, message))

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

// formatPlain now uses getCallerInfo with proper skip
func (l *Logger) formatPlain(level LogLevel, message string) string {
	var (
		levelStr     = fmt.Sprintf("%-5s", level.String())
		callerInfo   string
		timestampStr = time.Now().Format(l.timestampFormat)
	)

	if l.enableCaller {
		callerInfo = l.getCallerInfo(4) // Skip 4 frames to get to the actual log call
		if callerInfo != "" {
			callerInfo = callerInfo + " "
		}
	}

	return fmt.Sprintf("%s [%s] %s%s\n",
		timestampStr,
		levelStr,
		callerInfo,
		message,
	)
}

// formatJSON now uses getCallerInfo with proper skip
func (l *Logger) formatJSON(level LogLevel, message string) string {
	logEntry := struct {
		Timestamp string `json:"timestamp"`
		Level     string `json:"level"`
		Caller    string `json:"caller,omitempty"`
		Message   string `json:"message"`
	}{
		Timestamp: time.Now().Format(l.timestampFormat),
		Level:     level.String(),
		Message:   message,
	}

	if l.enableCaller {
		if caller := l.getCallerInfo(4); caller != "" {
			logEntry.Caller = caller
		}
	}

	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal log entry: %v"}`, err)
	}
	return string(jsonData) + "\n"
}

func (l *Logger) formatCLF(level LogLevel, message string) string {
	// Common Log Format: host ident authuser date request status bytes
	// Simplified version for general logging
	return fmt.Sprintf("%s - - [%s] %q %s\n",
		"localhost", // Could be made configurable
		time.Now().Format("02/Jan/2006:15:04:05 -0700"),
		message,
		level.String(),
	)
}

// Update all other format methods to use getCallerInfo with proper skip
func (l *Logger) formatGELF(level LogLevel, message string) string {
	gelf := map[string]interface{}{
		"version":       "1.1",
		"host":          "localhost",
		"short_message": message,
		"timestamp":     float64(time.Now().UnixNano()) / 1e9,
		"level":         int32(level),
	}

	if l.enableCaller {
		if caller := l.getCallerInfo(4); caller != "" {
			gelf["_caller"] = caller
		}
	}

	jsonData, err := json.Marshal(gelf)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal GELF entry: %v"}`, err)
	}
	return string(jsonData) + "\n"
}

func (l *Logger) formatLogfmt(level LogLevel, message string) string {
	var buf strings.Builder

	// Timestamp
	buf.WriteString(fmt.Sprintf("ts=%q ", time.Now().Format(l.timestampFormat)))

	// Level
	buf.WriteString(fmt.Sprintf("level=%s ", strings.ToLower(level.String())))

	// Message
	buf.WriteString(fmt.Sprintf("msg=%q ", message))

	// Caller info
	if l.enableCaller {
		if caller := l.getCallerInfo(4); caller != "" {
			buf.WriteString(fmt.Sprintf("caller=%q ", caller))
		}
	}

	// Custom fields
	for k, v := range l.formatConfig.CustomFields {
		buf.WriteString(fmt.Sprintf("%s=%v ", k, v))
	}

	return strings.TrimSpace(buf.String()) + "\n"
}

func (l *Logger) formatCSV(level LogLevel, message string) string {
	var buf strings.Builder

	if l.formatConfig.CSVHeaders && !l.csvHeadersWritten {
		buf.WriteString("timestamp,level,message,caller\n")
		l.csvHeadersWritten = true
	}

	// Timestamp
	buf.WriteString(fmt.Sprintf("%q%c", time.Now().Format(l.timestampFormat), l.formatConfig.CSVDelimiter))

	// Level
	buf.WriteString(fmt.Sprintf("%q%c", level.String(), l.formatConfig.CSVDelimiter))

	// Message
	buf.WriteString(fmt.Sprintf("%q%c", message, l.formatConfig.CSVDelimiter))

	// Caller info
	if l.enableCaller {
		if caller := l.getCallerInfo(4); caller != "" {
			buf.WriteString(fmt.Sprintf("%q", caller))
		}
	}

	return buf.String() + "\n"
}

func (l *Logger) formatXML(level LogLevel, message string) string {
	type LogEntry struct {
		Timestamp string                 `xml:"timestamp"`
		Level     string                 `xml:"level"`
		Message   string                 `xml:"message"`
		Caller    string                 `xml:"caller,omitempty"`
		Custom    map[string]interface{} `xml:"custom>field"`
	}

	entry := LogEntry{
		Timestamp: time.Now().Format(l.timestampFormat),
		Level:     level.String(),
		Message:   message,
		Custom:    l.formatConfig.CustomFields,
	}

	if l.enableCaller {
		if caller := l.getCallerInfo(4); caller != "" {
			entry.Caller = caller
		}
	}

	var xmlData []byte
	var err error

	if l.formatConfig.PrettyPrint {
		xmlData, err = xml.MarshalIndent(entry, "", "  ")
	} else {
		xmlData, err = xml.Marshal(entry)
	}

	if err != nil {
		return fmt.Sprintf("<error>failed to marshal XML entry: %v</error>\n", err)
	}

	return string(xmlData) + "\n"
}

func (l *Logger) formatCEF(level LogLevel, message string) string {
	// Common Event Format for security logging
	return fmt.Sprintf("CEF:0|GourdianLogger|Logger|1.0|%d|%s|%d|%s\n",
		level,
		message,
		time.Now().Unix(),
		l.getCEFExtensions(),
	)
}

func (l *Logger) getCEFExtensions() string {
	var exts []string

	if l.enableCaller {
		if caller := l.getCallerInfo(4); caller != "" {
			exts = append(exts, fmt.Sprintf("cs1=%s", caller))
		}
	}

	for k, v := range l.formatConfig.CustomFields {
		exts = append(exts, fmt.Sprintf("%s=%v", k, v))
	}

	return strings.Join(exts, " ")
}

// getCallerInfo retrieves the correct caller information with proper depth
func (l *Logger) getCallerInfo(skip int) string {
	// We need to skip:
	// 1. This function (getCallerInfo)
	// 2. The formatMessage function
	// 3. The log function
	// 4. The actual logging method (Debug, Info, etc.)
	// So default skip is 4, but we allow customization
	if skip <= 0 {
		skip = 4 // Default skip for normal logging calls
	}

	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}

	// Get just the file name
	fileName := filepath.Base(file)

	// Get the function name
	funcName := runtime.FuncForPC(pc).Name()

	// Simplify the function name by removing package path
	if lastSlash := strings.LastIndex(funcName, "/"); lastSlash >= 0 {
		funcName = funcName[lastSlash+1:]
	}

	// Remove any type information if it's a method
	if dot := strings.LastIndex(funcName, "."); dot >= 0 {
		funcName = funcName[dot+1:]
	}

	return fmt.Sprintf("%s:%d:%s", fileName, line, funcName)
}

func (l *Logger) formatMessage(level LogLevel, message string) string {
	switch l.format {
	case FormatJSON:
		return l.formatJSON(level, message)
	case FormatCLF:
		return l.formatCLF(level, message)
	case FormatGELF:
		return l.formatGELF(level, message)
	case FormatLogfmt:
		return l.formatLogfmt(level, message)
	case FormatCSV:
		return l.formatCSV(level, message)
	case FormatXML:
		return l.formatXML(level, message)
	case FormatCEF:
		return l.formatCEF(level, message)
	default:
		return l.formatPlain(level, message)
	}
}

// log is the internal logging function that routes to sync or async
func (l *Logger) log(level LogLevel, message string) {
	if l.asyncQueue != nil {
		select {
		case l.asyncQueue <- &logEntry{level, message}:
		default:
			// Fallback to synchronous if buffer is full
			l.processLogEntry(level, message)
		}
	} else {
		l.processLogEntry(level, message)
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

// SetLogLevel changes the minimum log level
func (l *Logger) SetLogLevel(level LogLevel) {
	l.level.Store(int32(level))
}

// GetLogLevel returns the current log level
func (l *Logger) GetLogLevel() LogLevel {
	return LogLevel(l.level.Load())
}

// AddOutput adds a new output destination
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

// RemoveOutput removes an output destination
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

// Close cleanly shuts down the logger
func (l *Logger) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	// Stop background workers
	close(l.rotateCloseChan)
	if l.asyncCloseChan != nil {
		close(l.asyncCloseChan)
	}
	l.wg.Wait()

	// Close file and clean up
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
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

// WithConfig creates a new logger from JSON config
func WithConfig(jsonConfig string) (*Logger, error) {
	// First, validate the JSON is valid
	if !json.Valid([]byte(jsonConfig)) {
		return nil, fmt.Errorf("invalid JSON config")
	}

	var config LoggerConfig
	if err := json.Unmarshal([]byte(jsonConfig), &config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Set default logs directory if not specified
	if config.LogsDir == "" {
		config.LogsDir = defaultLogsDir
	}

	// Ensure logs directory exists
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

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse log level
	level, err := ParseLogLevel(aux.LogLevelStr)
	if err != nil {
		return err
	}
	lc.LogLevel = level

	// Parse format
	if aux.FormatStr != "" {
		switch strings.ToUpper(aux.FormatStr) {
		case "PLAIN":
			lc.Format = FormatPlain
		case "JSON":
			lc.Format = FormatJSON
		case "CLF":
			lc.Format = FormatCLF
		case "GELF":
			lc.Format = FormatGELF
		default:
			return fmt.Errorf("invalid log format: %s", aux.FormatStr)
		}
	}

	return nil
}

// Flush ensures all buffered logs are written (for async mode)
func (l *Logger) Flush() {
	if l.asyncQueue != nil {
		// Wait for queue to drain
		for len(l.asyncQueue) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// IsClosed checks if logger is closed
func (l *Logger) IsClosed() bool {
	return l.closed.Load()
}
