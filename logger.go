package gourdianlogger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
)

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

// Debug logs a message at DEBUG level.
//
// Parameters:
//   - v: Values to log, will be converted to string using fmt.Sprint
//
// Example:
//
//	logger.Debug("This is a debug message")
//	logger.Debug("User", userID, "logged in")
func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, fmt.Sprint(v...))
}

// Info logs a message at INFO level.
//
// Parameters:
//   - v: Values to log, will be converted to string using fmt.Sprint
//
// Example:
//
//	logger.Info("Application started")
//	logger.Info("User", userID, "logged in")
func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, fmt.Sprint(v...))
}

// Warn logs a message at WARN level.
//
// Parameters:
//   - v: Values to log, will be converted to string using fmt.Sprint
//
// Example:
//
//	logger.Warn("Disk space is low")
//	logger.Warn("Invalid configuration for", configKey)
func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, fmt.Sprint(v...))
}

// Error logs a message at ERROR level.
//
// Parameters:
//   - v: Values to log, will be converted to string using fmt.Sprint
//
// Example:
//
//	logger.Error("Failed to connect to database")
//	logger.Error("File not found:", filename)
func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, fmt.Sprint(v...))
}

// Fatal logs a message at FATAL level and terminates the program.
//
// Parameters:
//   - v: Values to log, will be converted to string using fmt.Sprint
//
// Note:
//
//	This method calls os.Exit(1) after logging
//
// Example:
//
//	logger.Fatal("Critical error, shutting down")
//	logger.Fatal("Database connection failed, cannot continue")
func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, fmt.Sprint(v...))
	os.Exit(1)
}

// Debugf logs a formatted message at DEBUG level.
//
// Parameters:
//   - format: Printf-style format string
//   - v: Values for the format string
//
// Example:
//
//	logger.Debugf("User %s logged in from %s", userID, ipAddress)
//	logger.Debugf("Processing took %.2f seconds", duration.Seconds())
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, v...))
}

// Infof logs a formatted message at INFO level.
//
// Parameters:
//   - format: Printf-style format string
//   - v: Values for the format string
//
// Example:
//
//	logger.Infof("Server started on %s:%d", host, port)
//	logger.Infof("Processed %d records in %s", count, time.Since(start))
func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, v...))
}

// Warnf logs a formatted message at WARN level.
//
// Parameters:
//   - format: Printf-style format string
//   - v: Values for the format string
//
// Example:
//
//	logger.Warnf("Low disk space: %.1fGB remaining", availableGB)
//	logger.Warnf("Unexpected value %v for parameter %s", value, paramName)
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, v...))
}

// Errorf logs a formatted message at ERROR level.
//
// Parameters:
//   - format: Printf-style format string
//   - v: Values for the format string
//
// Example:
//
//	logger.Errorf("Failed to process file %s: %v", filename, err)
//	logger.Errorf("Database query failed: %s", query)
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, v...))
}

// Fatalf logs a formatted message at FATAL level and terminates the program.
//
// Parameters:
//   - format: Printf-style format string
//   - v: Values for the format string
//
// Note:
//
//	This method calls os.Exit(1) after logging
//
// Example:
//
//	logger.Fatalf("Critical error in %s: %v", functionName, err)
//	logger.Fatalf("Cannot start server: %s", err)
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Close closes the logger and its underlying file.
//
// This method:
// - Acquires write lock to ensure safety
// - Closes the underlying file handle
//
// Returns:
//   - error: Any error encountered while closing
//
// Thread safety:
//
//	Protected by mutex for concurrent access
//
// Example:
//
//	logger, err := NewGourdianLogger(config)
//	if err != nil {
//	    panic(err)
//	}
//	defer logger.Close()
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
