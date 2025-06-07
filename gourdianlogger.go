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

type LoggerConfig struct {
	BackupCount     int                    `json:"backup_count"`
	BufferSize      int                    `json:"buffer_size"`
	AsyncWorkers    int                    `json:"async_workers"`
	MaxLogRate      int                    `json:"max_log_rate"`
	EnableCaller    bool                   `json:"enable_caller"`
	EnableFallback  bool                   `json:"enable_fallback"`
	PrettyPrint     bool                   `json:"pretty_print"`
	MaxBytes        int64                  `json:"max_bytes"`
	Filename        string                 `json:"filename"`
	TimestampFormat string                 `json:"timestamp_format"`
	LogsDir         string                 `json:"logs_dir"`
	FormatStr       string                 `json:"format"`
	LogLevel        LogLevel               `json:"log_level"`
	CustomFields    map[string]interface{} `json:"custom_fields"`
	Outputs         []io.Writer            `json:"-"`
	LogFormat       LogFormat              `json:"-"`
	ErrorHandler    func(error)            `json:"-"`
}

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

	if config.MaxLogRate > 0 {
		logger.rateLimiter = rate.NewLimiter(rate.Limit(config.MaxLogRate), config.MaxLogRate)
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

func (l *Logger) writeBatch(buffers []*bytes.Buffer) {
	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	for _, buf := range buffers {
		if l.closed.Load() {
			fmt.Fprintf(os.Stderr, "Logger closed. Message: %s", buf.String())
			l.bufferPool.Put(buf)
			continue
		}

		if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
			l.handleError(fmt.Errorf("log write error: %w", err))
			if l.fallbackWriter != nil {
				fmt.Fprintf(l.fallbackWriter, "FALLBACK LOG: %s", buf.String())
			}
		}
		l.bufferPool.Put(buf)
	}
}

func (l *Logger) writeBuffer(buf *bytes.Buffer) {
	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	if l.closed.Load() {
		fmt.Fprintf(os.Stderr, "Logger closed. Message: %s", buf.String())
		l.bufferPool.Put(buf)
		return
	}

	if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
		l.handleError(fmt.Errorf("log write error: %w", err))
		if l.fallbackWriter != nil {
			fmt.Fprintf(l.fallbackWriter, "FALLBACK LOG: %s", buf.String())
		}
	}
	l.bufferPool.Put(buf)
}

func (l *Logger) handleError(err error) {
	if l.errorHandler != nil {
		l.errorHandler(err)
	} else if l.fallbackWriter != nil {
		fmt.Fprintf(l.fallbackWriter, "LOGGER ERROR: %v\n", err)
	}
}

func (l *Logger) formatPlain(level LogLevel, message, callerInfo string, fields map[string]interface{}) string {
	var builder strings.Builder
	builder.Grow(256) // Pre-allocate space for common case

	builder.WriteString(time.Now().Format(l.timestampFormat))
	builder.WriteByte(' ')
	builder.WriteByte('[')
	builder.WriteString(level.String())
	builder.WriteByte(']')
	builder.WriteByte(' ')

	if callerInfo != "" {
		builder.WriteString(callerInfo)
		builder.WriteByte(':')
	}

	builder.WriteString(message)

	if len(fields) > 0 {
		builder.WriteString(" {")
		first := true
		for k, v := range fields {
			if !first {
				builder.WriteString(", ")
			}
			first = false
			builder.WriteString(k)
			builder.WriteByte('=')
			fmt.Fprintf(&builder, "%v", v)
		}
		builder.WriteByte('}')
	}

	builder.WriteByte('\n')
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

	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return ""
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	fullFnName := fn.Name()
	if lastSlash := strings.LastIndex(fullFnName, "/"); lastSlash >= 0 {
		fullFnName = fullFnName[lastSlash+1:]
	}

	return fmt.Sprintf("%s:%d:%s", filepath.Base(file), line, fullFnName)
}

func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if l.closed.Load() || l.paused.Load() {
		return
	}

	if l.rateLimiter != nil && !l.rateLimiter.Allow() {
		return
	}

	currentLevel := LogLevel(l.level.Load())
	if l.dynamicLevelFn != nil {
		currentLevel = l.dynamicLevelFn()
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
			fmt.Fprintf(os.Stderr, "Logger closed. Message: %s", formatted)
			return
		}
		if _, err := l.multiWriter.Write([]byte(formatted)); err != nil {
			l.handleError(fmt.Errorf("log write error: %w", err))
			if l.fallbackWriter != nil {
				fmt.Fprintf(l.fallbackWriter, "FALLBACK LOG: %s", formatted)
			}
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

	oldFile := l.file
	if err := oldFile.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	base := strings.TrimSuffix(l.baseFilename, ".log")
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_%s.log", base, timestamp)

	if err := os.Rename(l.baseFilename, backupPath); err != nil {
		file, openErr := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			return fmt.Errorf("failed to rename log file (%v) and couldn't reopen original (%v)", err, openErr)
		}
		l.file = file
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	file, err := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	l.file = file

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

func (l *Logger) Flush() {
	if l.asyncQueue != nil {
		for len(l.asyncQueue) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
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

	if l.asyncCloseChan != nil {
		close(l.asyncCloseChan)
	}
	close(l.rotateCloseChan)

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Timeout reached, proceed with closing anyway
	}

	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) IsClosed() bool {
	return l.closed.Load()
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
	// l.Flush()
	// os.Exit(1)
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
	// l.Flush()
	// os.Exit(1)
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
	// l.Flush()
	// os.Exit(1)
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
	// l.Flush()
	// os.Exit(1)
}
