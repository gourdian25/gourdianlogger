package gourdianlogger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Test Constants
const (
	testLogDir    = "test_logs"
	testLogPrefix = "test_logger"
)

// Helper function to clean up test files
func cleanupTestFiles(t *testing.T) {
	t.Helper()
	err := os.RemoveAll(testLogDir)
	if err != nil {
		t.Fatalf("Failed to clean up test directory: %v", err)
	}
}

// TestLogLevelString tests the String method of LogLevel
func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
		{LogLevel(99), "DEBUG"}, // Unknown level defaults to DEBUG
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestParseLogLevel tests the ParseLogLevel function
func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
		wantErr  bool
	}{
		{"debug", DEBUG, false},
		{"INFO", INFO, false},
		{"warn", WARN, false},
		{"warning", WARN, false},
		{"ERROR", ERROR, false},
		{"fatal", FATAL, false},
		{"unknown", DEBUG, true},
		{"", DEBUG, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLogLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDefaultConfig tests the DefaultConfig function
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Filename != "app" {
		t.Errorf("DefaultConfig Filename = %v, want %v", config.Filename, "app")
	}
	if config.MaxBytes != defaultMaxBytes {
		t.Errorf("DefaultConfig MaxBytes = %v, want %v", config.MaxBytes, defaultMaxBytes)
	}
	if config.BackupCount != defaultBackupCount {
		t.Errorf("DefaultConfig BackupCount = %v, want %v", config.BackupCount, defaultBackupCount)
	}
	if config.LogLevel != DEBUG {
		t.Errorf("DefaultConfig LogLevel = %v, want %v", config.LogLevel, DEBUG)
	}
	if config.TimestampFormat != defaultTimestampFormat {
		t.Errorf("DefaultConfig TimestampFormat = %v, want %v", config.TimestampFormat, defaultTimestampFormat)
	}
	if config.LogsDir != defaultLogsDir {
		t.Errorf("DefaultConfig LogsDir = %v, want %v", config.LogsDir, defaultLogsDir)
	}
}

// TestNewGourdianLogger tests the logger creation
func TestNewGourdianLogger(t *testing.T) {
	defer cleanupTestFiles(t)

	tests := []struct {
		name    string
		config  LoggerConfig
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: LoggerConfig{
				Filename:    testLogPrefix,
				LogsDir:     testLogDir,
				BackupCount: 3,
			},
			wantErr: false,
		},
		{
			name: "Invalid Directory",
			config: LoggerConfig{
				Filename:    testLogPrefix,
				LogsDir:     "/invalid/path",
				BackupCount: 3,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGourdianLogger(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGourdianLogger() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoggingMethods tests all the logging methods
func TestLoggingMethods(t *testing.T) {
	defer cleanupTestFiles(t)

	// Create a buffer to capture output
	var buf bytes.Buffer

	config := LoggerConfig{
		Filename:    testLogPrefix,
		LogsDir:     testLogDir,
		BackupCount: 1,
		Outputs:     []io.Writer{&buf},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	tests := []struct {
		name     string
		logFunc  func()
		contains string
		level    LogLevel
	}{
		{
			name: "Debug",
			logFunc: func() {
				logger.Debug("test debug message")
			},
			contains: "test debug message",
			level:    DEBUG,
		},
		{
			name: "Info",
			logFunc: func() {
				logger.Info("test info message")
			},
			contains: "test info message",
			level:    INFO,
		},
		{
			name: "Warn",
			logFunc: func() {
				logger.Warn("test warn message")
			},
			contains: "test warn message",
			level:    WARN,
		},
		{
			name: "Error",
			logFunc: func() {
				logger.Error("test error message")
			},
			contains: "test error message",
			level:    ERROR,
		},
		{
			name: "Debugf",
			logFunc: func() {
				logger.Debugf("formatted %s", "debug")
			},
			contains: "formatted debug",
			level:    DEBUG,
		},
		{
			name: "Infof",
			logFunc: func() {
				logger.Infof("formatted %s", "info")
			},
			contains: "formatted info",
			level:    INFO,
		},
		{
			name: "Warnf",
			logFunc: func() {
				logger.Warnf("formatted %s", "warn")
			},
			contains: "formatted warn",
			level:    WARN,
		},
		{
			name: "Errorf",
			logFunc: func() {
				logger.Errorf("formatted %s", "error")
			},
			contains: "formatted error",
			level:    ERROR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			output := buf.String()
			if !strings.Contains(output, tt.contains) {
				t.Errorf("Expected log to contain %q, got %q", tt.contains, output)
			}
			if !strings.Contains(output, tt.level.String()) {
				t.Errorf("Expected log level %q in output, got %q", tt.level.String(), output)
			}
		})
	}
}

// TestLogLevelFiltering tests that logs are filtered by level
func TestLogLevelFiltering(t *testing.T) {
	defer cleanupTestFiles(t)

	var buf bytes.Buffer

	config := LoggerConfig{
		Filename:    testLogPrefix,
		LogsDir:     testLogDir,
		BackupCount: 1,
		Outputs:     []io.Writer{&buf},
		LogLevel:    WARN, // Only WARN and above should be logged
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log messages at all levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	// Check that DEBUG and INFO messages are filtered out
	if strings.Contains(output, "debug message") {
		t.Error("DEBUG message was not filtered out")
	}
	if strings.Contains(output, "info message") {
		t.Error("INFO message was not filtered out")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("WARN message was filtered out")
	}
	if !strings.Contains(output, "error message") {
		t.Error("ERROR message was filtered out")
	}
}

// TestLogRotation tests log rotation functionality
func TestLogRotation(t *testing.T) {
	defer cleanupTestFiles(t)

	config := LoggerConfig{
		Filename:    testLogPrefix,
		LogsDir:     testLogDir,
		MaxBytes:    100, // Very small size to force rotation
		BackupCount: 2,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write enough logs to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info(strings.Repeat("a", 10)) // Each log is ~50 bytes with metadata
	}

	// Manually trigger rotation
	logger.mu.Lock()
	err = logger.rotateLogFiles()
	logger.mu.Unlock()
	if err != nil {
		t.Fatalf("Rotation failed: %v", err)
	}

	// Check backup files
	backupFiles, err := filepath.Glob(filepath.Join(testLogDir, testLogPrefix+"_*.log"))
	if err != nil {
		t.Fatalf("Failed to list backup files: %v", err)
	}

	if len(backupFiles) == 0 {
		t.Error("No backup files were created")
	}

	// Check backup count enforcement
	for i := 0; i < 10; i++ {
		logger.Info(strings.Repeat("b", 10))
	}

	// Trigger another rotation
	logger.mu.Lock()
	err = logger.rotateLogFiles()
	logger.mu.Unlock()
	if err != nil {
		t.Fatalf("Second rotation failed: %v", err)
	}

	backupFiles, err = filepath.Glob(filepath.Join(testLogDir, testLogPrefix+"_*.log"))
	if err != nil {
		t.Fatalf("Failed to list backup files: %v", err)
	}

	if len(backupFiles) > config.BackupCount {
		t.Errorf("Expected max %d backup files, got %d", config.BackupCount, len(backupFiles))
	}
}

// TestAsyncLogging tests asynchronous logging functionality
func TestAsyncLogging(t *testing.T) {
	defer cleanupTestFiles(t)

	var buf bytes.Buffer

	config := LoggerConfig{
		Filename:     testLogPrefix,
		LogsDir:      testLogDir,
		BackupCount:  1,
		Outputs:      []io.Writer{&buf},
		BufferSize:   100,
		AsyncWorkers: 2,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log many messages quickly
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("message %d", i))
		}(i)
	}
	wg.Wait()

	// Flush and close
	logger.Flush()
	logger.Close()

	// Verify all messages were logged
	output := buf.String()
	for i := 0; i < 100; i++ {
		if !strings.Contains(output, fmt.Sprintf("message %d", i)) {
			t.Errorf("Missing message %d in output", i)
		}
	}
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	defer cleanupTestFiles(t)

	config := LoggerConfig{
		Filename:    testLogPrefix,
		LogsDir:     testLogDir,
		BackupCount: 1,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log a message
	logger.Info("test message")

	// Close the logger
	if err := logger.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify closed state
	if !logger.IsClosed() {
		t.Error("Logger should be closed")
	}

	// Attempt to log after close
	logger.Info("should not be logged")
}

// TestWithConfig tests creating a logger from JSON config
func TestWithConfig(t *testing.T) {
	defer cleanupTestFiles(t)

	jsonConfig := `{
		"filename": "json_config_test",
		"logs_dir": "test_logs",
		"max_bytes": 1024,
		"backup_count": 3,
		"log_level": "WARN",
		"timestamp_format": "2006-01-02",
		"enable_caller": false,
		"buffer_size": 50,
		"async_workers": 2
	}`

	logger, err := WithConfig(jsonConfig)
	if err != nil {
		t.Fatalf("WithConfig() error = %v", err)
	}
	defer logger.Close()

	// Verify configuration
	if logger.GetLogLevel() != WARN {
		t.Errorf("Expected log level WARN, got %v", logger.GetLogLevel())
	}

	// Test that the log file was created
	logPath := filepath.Join(testLogDir, "json_config_test.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created at %s", logPath)
	}
}

// TestAddRemoveOutput tests adding and removing output writers
func TestAddRemoveOutput(t *testing.T) {
	defer cleanupTestFiles(t)

	var buf1, buf2 bytes.Buffer

	config := LoggerConfig{
		Filename:    testLogPrefix,
		LogsDir:     testLogDir,
		BackupCount: 1,
		Outputs:     []io.Writer{&buf1},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add a new output
	logger.AddOutput(&buf2)

	// Log a message
	msg := "test add/remove output"
	logger.Info(msg)

	// Check both buffers
	if !strings.Contains(buf1.String(), msg) {
		t.Error("Original output didn't receive message")
	}
	if !strings.Contains(buf2.String(), msg) {
		t.Error("New output didn't receive message")
	}

	// Remove the original output
	logger.RemoveOutput(&buf1)

	// Clear buffers
	buf1.Reset()
	buf2.Reset()

	// Log another message
	msg2 := "after remove"
	logger.Info(msg2)

	// Check buffers
	if strings.Contains(buf1.String(), msg2) {
		t.Error("Removed output still received message")
	}
	if !strings.Contains(buf2.String(), msg2) {
		t.Error("Remaining output didn't receive message")
	}
}

// TestCallerInfo tests the caller information inclusion
func TestCallerInfo(t *testing.T) {
	defer cleanupTestFiles(t)

	var buf bytes.Buffer

	config := LoggerConfig{
		Filename:     testLogPrefix,
		LogsDir:      testLogDir,
		BackupCount:  1,
		Outputs:      []io.Writer{&buf},
		EnableCaller: true,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log a message
	logger.Info("test caller info")

	output := buf.String()
	if !strings.Contains(output, "gourdianlogger_test.go") {
		t.Error("Expected caller info in log output")
	}
}

// TestFatal tests that Fatal logs exit the program
func TestFatal(t *testing.T) {
	defer cleanupTestFiles(t)

	// This test needs to run in a subprocess because it calls os.Exit
	if os.Getenv("BE_CRASHER") == "1" {
		config := LoggerConfig{
			Filename:    testLogPrefix,
			LogsDir:     testLogDir,
			BackupCount: 1,
		}

		logger, err := NewGourdianLogger(config)
		if err != nil {
			fmt.Printf("Failed to create logger: %v\n", err)
			os.Exit(1)
		}
		defer logger.Close()

		logger.Fatal("fatal error")
		return
	}

	cmd := os.Command(os.Args[0], "-test.run=TestFatal")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	if e, ok := err.(*os.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

// TestConcurrentLogging tests concurrent access to the logger
func TestConcurrentLogging(t *testing.T) {
	defer cleanupTestFiles(t)

	var buf bytes.Buffer

	config := LoggerConfig{
		Filename:     testLogPrefix,
		LogsDir:      testLogDir,
		BackupCount:  1,
		Outputs:      []io.Writer{&buf},
		BufferSize:   100,
		AsyncWorkers: 4,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	iterations := 100

	// Test concurrent logging
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("concurrent message %d", i))
		}(i)
	}

	wg.Wait()
	logger.Flush()

	// Verify all messages were logged
	output := buf.String()
	for i := 0; i < iterations; i++ {
		if !strings.Contains(output, fmt.Sprintf("concurrent message %d", i)) {
			t.Errorf("Missing message %d in output", i)
		}
	}
}

// TestLogLevelChange tests changing log level at runtime
func TestLogLevelChange(t *testing.T) {
	defer cleanupTestFiles(t)

	var buf bytes.Buffer

	config := LoggerConfig{
		Filename:    testLogPrefix,
		LogsDir:     testLogDir,
		BackupCount: 1,
		Outputs:     []io.Writer{&buf},
		LogLevel:    INFO,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// DEBUG message should be filtered
	logger.Debug("debug message 1")
	if strings.Contains(buf.String(), "debug message 1") {
		t.Error("DEBUG message was not filtered")
	}

	// Change to DEBUG level
	logger.SetLogLevel(DEBUG)

	// DEBUG message should now appear
	logger.Debug("debug message 2")
	if !strings.Contains(buf.String(), "debug message 2") {
		t.Error("DEBUG message was filtered after level change")
	}
}

// TestBufferPool tests the buffer pool functionality
func TestBufferPool(t *testing.T) {
	defer cleanupTestFiles(t)

	config := LoggerConfig{
		Filename:    testLogPrefix,
		LogsDir:     testLogDir,
		BackupCount: 1,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Get a buffer from the pool
	buf := logger.bufferPool.Get().(*bytes.Buffer)
	if buf == nil {
		t.Fatal("Got nil buffer from pool")
	}

	// Use and return the buffer
	buf.WriteString("test")
	logger.bufferPool.Put(buf)

	// Get another buffer - should reuse the same one
	buf2 := logger.bufferPool.Get().(*bytes.Buffer)
	if buf2.Len() != 0 {
		t.Error("Buffer was not reset when returned to pool")
	}
}

// TestInvalidConfigs tests various invalid configurations
func TestInvalidConfigs(t *testing.T) {
	defer cleanupTestFiles(t)

	tests := []struct {
		name    string
		config  LoggerConfig
		wantErr bool
	}{
		{
			name: "Negative MaxBytes",
			config: LoggerConfig{
				Filename:    testLogPrefix,
				LogsDir:     testLogDir,
				MaxBytes:    -1,
				BackupCount: 1,
			},
			wantErr: false, // Should default to positive value
		},
		{
			name: "Zero BackupCount",
			config: LoggerConfig{
				Filename:    testLogPrefix,
				LogsDir:     testLogDir,
				MaxBytes:    100,
				BackupCount: 0,
			},
			wantErr: false, // Should default to positive value
		},
		{
			name: "Empty Filename",
			config: LoggerConfig{
				Filename:    "",
				LogsDir:     testLogDir,
				MaxBytes:    100,
				BackupCount: 1,
			},
			wantErr: false, // Should default to "app"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGourdianLogger(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGourdianLogger() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
