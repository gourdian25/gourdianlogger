package gourdianlogger

import (
	"bytes"
	"errors"
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

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[l]
}

func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	default:
		return DEBUG, errors.New("invalid log level: " + level)
	}
}

type Logger struct {
	mu              sync.RWMutex
	level           LogLevel
	baseFilename    string
	maxBytes        int64
	backupCount     int
	file            *os.File
	multiWriter     io.Writer
	bufferPool      *sync.Pool
	timestampFormat string
	outputs         []io.Writer
}

type LoggerConfig struct {
	Filename        string
	MaxBytes        int64
	BackupCount     int
	LogLevel        LogLevel
	TimestampFormat string
	Outputs         []io.Writer
}

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
func (l *Logger) log(level LogLevel, message string) {
	if level < l.level {
		return
	}

	formattedMsg := l.formatLogMessage(level, message)

	buf := l.bufferPool.Get().(*bytes.Buffer)
	defer l.bufferPool.Put(buf)
	buf.Reset()
	buf.WriteString(formattedMsg)

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.checkFileSize(); err != nil {
		fmt.Fprintf(os.Stderr, "Log rotation error: %v\n", err)
	}

	if _, err := l.multiWriter.Write(buf.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log message: %v\nOriginal message: %s", err, formattedMsg)
		if level == FATAL {
			os.Exit(1)
		}
	}
}
func (l *Logger) SetLogLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

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
	os.Exit(1)
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
	os.Exit(1)
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

func (l *Logger) AddOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputs = append(l.outputs, output)
	l.multiWriter = io.MultiWriter(l.outputs...)
}

func (l *Logger) formatLogMessage(level LogLevel, message string) string {
	pc, file, line, ok := runtime.Caller(4)
	if !ok {
		file = "unknown"
		line = 0
	}

	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		fullFunc := fn.Name()
		if lastSlash := strings.LastIndexByte(fullFunc, '/'); lastSlash >= 0 {
			fullFunc = fullFunc[lastSlash+1:]
		}
		if lastDot := strings.LastIndexByte(fullFunc, '.'); lastDot >= 0 {
			funcName = fullFunc[lastDot+1:]
		} else {
			funcName = fullFunc
		}
	}

	file = filepath.Base(file)

	now := time.Now()
	return fmt.Sprintf("%s [%s] %s:%d (%s): %s\n",
		now.Format(l.timestampFormat),
		level,
		file,
		line,
		funcName,
		message,
	)
}

func (l *Logger) checkFileSize() error {
	fi, err := l.file.Stat()
	if err != nil {
		return err
	}

	if fi.Size() >= l.maxBytes {
		return l.rotateLogFiles()
	}

	return nil
}

func (l *Logger) rotateLogFiles() error {
	if err := l.file.Close(); err != nil {
		return err
	}

	baseWithoutExt := strings.TrimSuffix(l.baseFilename, ".log")
	currentTime := time.Now().Format("20060102_150405")
	newLogFilePath := fmt.Sprintf("%s_%s.log", baseWithoutExt, currentTime)

	if err := os.Rename(l.baseFilename, newLogFilePath); err != nil {
		return err
	}

	backupFiles, _ := filepath.Glob(baseWithoutExt + "_*.log")

	sort.Strings(backupFiles)

	if len(backupFiles) > l.backupCount {
		for _, oldFile := range backupFiles[:len(backupFiles)-l.backupCount] {
			os.Remove(oldFile)
		}
	}

	file, err := os.OpenFile(l.baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.file = file

	outputs := []io.Writer{os.Stdout, file}
	if len(l.outputs) > 0 {
		outputs = append(outputs, l.outputs[2:]...) // Skip stdout and previous file
	}
	l.outputs = outputs
	l.multiWriter = io.MultiWriter(outputs...)

	return nil
}
