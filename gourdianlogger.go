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

	"github.com/charmbracelet/lipgloss"
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

var (
	// Default values
	defaultMaxBytes        int64  = 10 * 1024 * 1024 // 10MB
	defaultBackupCount     int    = 5
	defaultTimestampFormat string = "2006-01-02 15:04:05.000000"
	defaultLogsDir         string = "logs"
)

// Color styles for different log levels
var (
	debugStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")) // Purple
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#43BF6D")) // Green
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")) // Orange
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")) // Pink/Red
	fatalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")). // White
			Background(lipgloss.Color("#FF0000")). // Red
			Bold(true)
	timestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888B7E")) // Gray
	callerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A550DF")) // Purple
)

// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	Filename        string      `json:"filename"`         // Base filename for logs
	MaxBytes        int64       `json:"max_bytes"`        // Max file size before rotation
	BackupCount     int         `json:"backup_count"`     // Number of backups to keep
	LogLevel        LogLevel    `json:"log_level"`        // Minimum log level
	TimestampFormat string      `json:"timestamp_format"` // Custom timestamp format
	Outputs         []io.Writer `json:"-"`                // Additional outputs (not JSON serializable)
	LogsDir         string      `json:"logs_dir"`         // Directory for log files
	EnableCaller    bool        `json:"enable_caller"`    // Include caller info
	BufferSize      int         `json:"buffer_size"`      // Buffer size for async logging
	AsyncWorkers    int         `json:"async_workers"`    // Number of async workers
	EnableColor     bool        `json:"enable_color"`     // Enable colored output
	ShowBanner      bool        `json:"show_banner"`      // Show banner on initialization
}

// Logger is the main logging struct
type Logger struct {
	mu               sync.RWMutex   // Protects non-atomic operations
	level            atomic.Int32   // Atomic log level
	baseFilename     string         // Base log file path
	maxBytes         int64          // Max file size
	backupCount      int            // Number of backups
	file             *os.File       // Current log file
	multiWriter      io.Writer      // Combined output
	bufferPool       sync.Pool      // Reusable buffers
	timestampFormat  string         // Time format
	outputs          []io.Writer    // All outputs
	logsDir          string         // Log directory
	closed           atomic.Bool    // Atomic closed flag
	rotateChan       chan struct{}  // Rotation signal
	rotateCloseChan  chan struct{}  // Rotation stop signal
	wg               sync.WaitGroup // Background ops
	enableCaller     bool           // Include caller info
	asyncQueue       chan *logEntry // Async logging queue
	asyncCloseChan   chan struct{}  // Async stop signal
	asyncWorkerCount int            // Number of async workers
	enableColor      bool           // Enable colored output
}

// colorWriter is a custom writer that applies color to console output
type colorWriter struct{}

func (w colorWriter) Write(p []byte) (n int, err error) {
	// For the console, we'll let the formatMessage function handle coloring
	return os.Stdout.Write(p)
}

// showBanner displays the cute cat banner
func (l *Logger) showBanner() {
	cat := `
  猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪
  猫咪猫咪猫咪猫咪猫咪猫╭──────────────────────────────────────────────────╮猫咪猫咪猫咪猫咪猫咪猫
  猫咪猫咪猫咪猫咪猫咪猫│                                                  │猫咪猫咪猫咪猫咪猫咪猫
  猫咪猫咪猫咪猫咪猫咪猫│     Are you sure you want to eat marmalade?      │猫咪猫咪猫咪猫咪猫咪猫
  猫咪猫咪猫咪猫咪猫咪猫│                                                  │猫咪猫咪猫咪猫咪猫咪猫
  猫咪猫咪猫咪猫咪猫咪猫│                 Yes        Maybe                 │猫咪猫咪猫咪猫咪猫咪猫
  猫咪猫咪猫咪猫咪猫咪猫│                                                  │猫咪猫咪猫咪猫咪猫咪猫
  猫咪猫咪猫咪猫咪猫咪猫╰──────────────────────────────────────────────────╯猫咪猫咪猫咪猫咪猫咪猫
  猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪猫咪
`

	// Style the banner
	bannerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F25D94")). // Pink
		Background(lipgloss.Color("#FFF7DB")). // Cream
		Margin(1, 0, 1, 0)

	// Print the banner to stdout (not to log files)
	fmt.Println(bannerStyle.Render(cat))
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
		BufferSize:      0,    // Sync by default
		AsyncWorkers:    1,    // Default workers if async enabled
		EnableColor:     true, // Color enabled by default
		ShowBanner:      true, // Show banner by default
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

	logPath := filepath.Join(config.LogsDir, config.Filename+".log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Setup outputs with stdout and file as defaults
	outputs := []io.Writer{file} // File always first
	if config.EnableColor {
		outputs = append(outputs, colorWriter{}) // Our custom color writer for console
	} else {
		outputs = append(outputs, os.Stdout) // Plain stdout if color disabled
	}

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
		enableColor:      config.EnableColor,
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

	// Show banner if enabled
	if config.ShowBanner {
		logger.showBanner()
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

// formatMessage formats the log message with all metadata and color
func (l *Logger) formatMessage(level LogLevel, message string) string {
	var (
		levelStr     string
		callerInfo   string
		timestampStr = time.Now().Format(l.timestampFormat)
	)

	// Apply color to level if enabled
	if l.enableColor {
		switch level {
		case DEBUG:
			levelStr = debugStyle.Render(level.String())
		case INFO:
			levelStr = infoStyle.Render(level.String())
		case WARN:
			levelStr = warnStyle.Render(level.String())
		case ERROR:
			levelStr = errorStyle.Render(level.String())
		case FATAL:
			levelStr = fatalStyle.Render(level.String())
		}
		timestampStr = timestampStyle.Render(timestampStr)
	} else {
		levelStr = level.String()
	}

	// Get caller info if enabled
	if l.enableCaller {
		pc, file, line, ok := runtime.Caller(3) // Adjusted depth for wrapper methods
		if ok {
			funcName := "unknown"
			if fn := runtime.FuncForPC(pc); fn != nil {
				name := fn.Name()
				if lastSlash := strings.LastIndex(name, "/"); lastSlash >= 0 {
					name = name[lastSlash+1:]
				}
				if lastDot := strings.LastIndex(name, "."); lastDot >= 0 {
					funcName = name[lastDot+1:]
				} else {
					funcName = name
				}
			}
			callerInfo = fmt.Sprintf("%s:%d(%s)", filepath.Base(file), line, funcName)
			if l.enableColor {
				callerInfo = callerStyle.Render(callerInfo) + " "
			} else {
				callerInfo = callerInfo + " "
			}
		}
	}

	return fmt.Sprintf("%s [%s] %s%s\n",
		timestampStr,
		levelStr,
		callerInfo,
		message,
	)
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
	var config LoggerConfig
	if err := json.Unmarshal([]byte(jsonConfig), &config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return NewGourdianLogger(config)
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
