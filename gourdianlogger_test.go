package gourdianlogger

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// failingWriter is an io.Writer that always fails
type failingWriter struct{}

func (w *failingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("simulated write error")
}

// TestLogLevelString tests the String() method of LogLevel
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
		{LogLevel(99), "UNKNOWN"},
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
		{"DEBUG", DEBUG, false},
		{"debug", DEBUG, false},
		{"INFO", INFO, false},
		{"info", INFO, false},
		{"WARN", WARN, false},
		{"warn", WARN, false},
		{"WARNING", WARN, false},
		{"warning", WARN, false},
		{"ERROR", ERROR, false},
		{"error", ERROR, false},
		{"FATAL", FATAL, false},
		{"fatal", FATAL, false},
		{"INVALID", DEBUG, true},
		{"", DEBUG, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLogLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestLoggerConfigValidation tests the Validate method of LoggerConfig
func TestLoggerConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  LoggerConfig
		wantErr bool
	}{
		{
			name:    "ValidDefault",
			config:  LoggerConfig{},
			wantErr: false,
		},
		{
			name: "NegativeMaxBytes",
			config: LoggerConfig{
				MaxBytes: -1,
			},
			wantErr: true,
		},
		{
			name: "NegativeBackupCount",
			config: LoggerConfig{
				BackupCount: -1,
			},
			wantErr: true,
		},
		{
			name: "NegativeBufferSize",
			config: LoggerConfig{
				BufferSize: -1,
			},
			wantErr: true,
		},
		{
			name: "NegativeAsyncWorkers",
			config: LoggerConfig{
				AsyncWorkers: -1,
			},
			wantErr: true,
		},
		{
			name: "NegativeMaxLogRate",
			config: LoggerConfig{
				MaxLogRate: -1,
			},
			wantErr: true,
		},
		{
			name: "InvalidFormatStr",
			config: LoggerConfig{
				FormatStr: "INVALID",
			},
			wantErr: true,
		},
		{
			name: "ValidFormatStr",
			config: LoggerConfig{
				FormatStr: "JSON",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoggerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNewGourdianLogger tests the logger initialization
func TestNewGourdianLogger(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		config  LoggerConfig
		wantErr bool
	}{
		{
			name: "DefaultConfig",
			config: LoggerConfig{
				LogsDir: tempDir,
			},
			wantErr: false,
		},
		{
			name: "InvalidDirectory",
			config: LoggerConfig{
				LogsDir: "/nonexistent/path",
			},
			wantErr: true,
		},
		{
			name: "WithCustomOutputs",
			config: LoggerConfig{
				LogsDir:  tempDir,
				Outputs:  []io.Writer{&bytes.Buffer{}},
				LogLevel: INFO,
			},
			wantErr: false,
		},
		{
			name: "WithAsyncConfig",
			config: LoggerConfig{
				LogsDir:      tempDir,
				BufferSize:   100,
				AsyncWorkers: 2,
			},
			wantErr: false,
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

// TestLogFormatting tests the formatting functions
func TestLogFormatting(t *testing.T) {
	// Create a test logger with controlled caller info
	logger := &Logger{
		timestampFormat: "2006-01-02 15:04:05.000000",
		enableCaller:    true,
		format:          FormatPlain,
	}

	// Create a test case struct
	type testCase struct {
		name     string
		format   LogFormat
		level    LogLevel
		message  string
		fields   map[string]interface{}
		expected []string
	}

	testCases := []testCase{
		{
			name:    "PlainFormat",
			format:  FormatPlain,
			level:   INFO,
			message: "test message",
			fields:  map[string]interface{}{"key": "value", "num": 42},
			expected: []string{
				"[INFO]",
				"test message",
				"key=value",
				"num=42",
			},
		},
		{
			name:    "JSONFormat",
			format:  FormatJSON,
			level:   INFO,
			message: "json message",
			fields:  map[string]interface{}{"key": "value", "num": 42},
			expected: []string{
				`"level":"INFO"`,
				`"message":"json message"`,
				`"key":"value"`,
				`"num":42`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger.format = tc.format

			// For testing purposes, we'll pass the caller info directly
			callerInfo := "file.go:42:function"
			var formatted string

			if tc.format == FormatJSON {
				formatted = logger.formatJSON(tc.level, tc.message, callerInfo, tc.fields)
			} else {
				formatted = logger.formatPlain(tc.level, tc.message, callerInfo, tc.fields)
			}

			// Verify all expected substrings are present
			for _, substr := range tc.expected {
				if !strings.Contains(formatted, substr) {
					t.Errorf("Formatted log missing expected substring: %q", substr)
				}
			}

			// For plain format, also verify the caller info appears
			if tc.format == FormatPlain {
				if !strings.Contains(formatted, callerInfo) {
					t.Error("Plain format log missing caller info")
				}
			}

			// For JSON format, verify the caller field appears
			if tc.format == FormatJSON && logger.enableCaller {
				if !strings.Contains(formatted, `"caller":"file.go:42:function"`) {
					t.Error("JSON format log missing caller info")
				}
			}
		})
	}
}

// TestLogLevelFiltering tests that logs are filtered by level
func TestLogLevelFiltering(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:  tempDir,
		Outputs:  []io.Writer{buf},
		LogLevel: WARN,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	logger.Flush()

	output := buf.String()

	if strings.Contains(output, "debug message") {
		t.Error("Debug message should not be logged when level is WARN")
	}
	if strings.Contains(output, "info message") {
		t.Error("Info message should not be logged when level is WARN")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should be logged when level is WARN")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should be logged when level is WARN")
	}
}

// TestAsyncLogging tests the async logging functionality
func TestAsyncLogging(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Outputs:      []io.Writer{buf},
		BufferSize:   100,
		AsyncWorkers: 1,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log enough messages to fill the buffer
	for i := 0; i < 150; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}

	// Give some time for messages to process
	time.Sleep(100 * time.Millisecond)

	// Close to flush all messages
	logger.Close()

	// Verify all messages were written
	output := buf.String()
	missing := 0
	for i := 0; i < 150; i++ {
		if !strings.Contains(output, fmt.Sprintf("message %d", i)) {
			missing++
		}
	}

	if missing > 0 {
		t.Errorf("Missing %d messages in output", missing)
	}
}

// TestLogRotation tests the log rotation functionality
func TestLogRotation(t *testing.T) {
	tempDir := t.TempDir()

	config := LoggerConfig{
		LogsDir:     tempDir,
		Filename:    "rotation_test",
		MaxBytes:    100, // Small size to force rotation
		BackupCount: 2,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write enough data to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info("This is a test message that should be long enough to trigger rotation")
	}
	logger.Flush()

	// Check backup files
	pattern := filepath.Join(tempDir, "rotation_test_*.log")
	backups, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Failed to find backup files: %v", err)
	}

	if len(backups) > config.BackupCount {
		t.Errorf("Expected max %d backups, got %d", config.BackupCount, len(backups))
	}
}

// TestAddRemoveOutput tests adding and removing outputs
func TestAddRemoveOutput(t *testing.T) {
	tempDir := t.TempDir()
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir: tempDir,
		Outputs: []io.Writer{buf1},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test initial output
	logger.Info("message 1")
	if buf1.String() == "" {
		t.Error("Initial output not working")
	}

	// Add second output
	logger.AddOutput(buf2)
	logger.Info("message 2")
	if buf2.String() == "" {
		t.Error("Added output not working")
	}

	// Remove first output
	logger.RemoveOutput(buf1)
	logger.Info("message 3")
	if strings.Count(buf1.String(), "message") != 2 {
		t.Error("RemoveOutput didn't work as expected")
	}
}

// TestErrorHandling tests error handling scenarios
func TestErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	errorBuf := &bytes.Buffer{}
	fallbackBuf := &bytes.Buffer{}

	// Create a custom error handler
	errorHandler := func(err error) {
		fmt.Fprintf(errorBuf, "ERROR: %v", err)
	}

	config := LoggerConfig{
		LogsDir:        tempDir,
		EnableFallback: true,
		ErrorHandler:   errorHandler,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Replace multiWriter with one that will fail
	logger.multiWriter = &failingWriter{}

	// Replace fallback writer for testing
	logger.fallbackWriter = fallbackBuf

	logger.Info("test message")
	logger.Flush()

	// Verify error handler was called
	if errorBuf.String() == "" {
		t.Error("Error handler was not called")
	}

	// Verify fallback was used
	if fallbackBuf.String() == "" {
		t.Error("Fallback writer was not used")
	}
}

// TestCloseBehavior tests logger close functionality
func TestCloseBehavior(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Outputs:      []io.Writer{buf},
		BufferSize:   100,
		AsyncWorkers: 1,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log some messages
	for i := 0; i < 10; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}

	// Close the logger
	err = logger.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify closed state
	if !logger.IsClosed() {
		t.Error("IsClosed() returned false after Close()")
	}

	// Try to log after close
	logger.Info("should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Error("Logger accepted messages after close")
	}

	// Double close should not panic
	err = logger.Close()
	if err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}

// TestPauseResume tests pause and resume functionality
func TestPauseResume(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir: tempDir,
		Outputs: []io.Writer{buf},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Pause logging
	logger.Pause()
	if !logger.IsPaused() {
		t.Error("IsPaused() returned false after Pause()")
	}

	// Log while paused
	logger.Info("paused message")
	logger.Flush()
	if strings.Contains(buf.String(), "paused message") {
		t.Error("Logger accepted messages while paused")
	}

	// Resume logging
	logger.Resume()
	if logger.IsPaused() {
		t.Error("IsPaused() returned true after Resume()")
	}

	// Log after resume
	logger.Info("resumed message")
	logger.Flush()
	if !strings.Contains(buf.String(), "resumed message") {
		t.Error("Logger didn't accept messages after resume")
	}
}

func TestCustomFields(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:   tempDir,
		Outputs:   []io.Writer{buf},
		LogFormat: FormatJSON,
		CustomFields: map[string]interface{}{
			"service": "test-service",
			"version": 1.0,
		},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("test message")
	logger.Flush()

	output := buf.String()

	if !strings.Contains(output, `"service":"test-service"`) {
		t.Error("JSON output missing custom service field")
	}
	if !strings.Contains(output, `"version":1`) {
		t.Error("JSON output missing custom version field")
	}
}

func TestBufferPool(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Outputs:      []io.Writer{buf},
		BufferSize:   100,
		AsyncWorkers: 1,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Get initial buffer from pool
	initialBuf := logger.bufferPool.Get().(*bytes.Buffer)
	initialCap := initialBuf.Cap()
	logger.bufferPool.Put(initialBuf)

	// Log enough messages to test pool usage
	for i := 0; i < 150; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}

	logger.Close()

	// Verify pool is being used by checking capacity consistency
	newBuf := logger.bufferPool.Get().(*bytes.Buffer)
	newCap := newBuf.Cap()
	logger.bufferPool.Put(newBuf)

	if initialCap != newCap {
		t.Errorf("Buffer pool capacity changed unexpectedly, was %d, now %d", initialCap, newCap)
	}
}

func TestLogMethodsWithFields(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:   tempDir,
		Outputs:   []io.Writer{buf},
		LogFormat: FormatJSON,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	fields := map[string]interface{}{
		"user":    "testuser",
		"request": "12345",
	}

	logger.DebugWithFields(fields, "debug message")
	logger.InfoWithFields(fields, "info message")
	logger.WarnWithFields(fields, "warn message")
	logger.ErrorWithFields(fields, "error message")

	logger.Flush()

	output := buf.String()

	for _, level := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		if !strings.Contains(output, fmt.Sprintf(`"level":"%s"`, level)) {
			t.Errorf("Missing %s level log", level)
		}
	}

	if !strings.Contains(output, `"user":"testuser"`) {
		t.Error("Missing user field in logs")
	}
	if !strings.Contains(output, `"request":"12345"`) {
		t.Error("Missing request field in logs")
	}
}

func TestLogFormatSwitching(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir: tempDir,
		Outputs: []io.Writer{buf},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test plain format
	logger.format = FormatPlain
	logger.Info("plain message")
	plainOutput := buf.String()
	buf.Reset()

	if !strings.Contains(plainOutput, "[INFO] plain message") {
		t.Error("Plain format not working as expected")
	}

	// Test JSON format
	logger.format = FormatJSON
	logger.Info("json message")
	jsonOutput := buf.String()

	if !strings.Contains(jsonOutput, `"level":"INFO"`) || !strings.Contains(jsonOutput, `"message":"json message"`) {
		t.Error("JSON format not working as expected")
	}
}

func TestFatalMethods(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir: tempDir,
		Outputs: []io.Writer{buf},
	}

	// Since Fatal calls os.Exit, we need to test this in a subprocess
	if os.Getenv("BE_FATAL") == "1" {
		logger, _ := NewGourdianLogger(config)
		logger.Fatal("fatal error occurred")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalMethods")
	cmd.Env = append(os.Environ(), "BE_FATAL=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestPrettyPrintJSON(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:     tempDir,
		Outputs:     []io.Writer{buf},
		LogFormat:   FormatJSON,
		PrettyPrint: true,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("pretty message")
	logger.Flush()

	output := buf.String()
	// Pretty print adds newlines and spaces
	if !strings.Contains(output, "\n  \"") {
		t.Error("Output is not pretty printed")
	}
}

func TestDynamicLogLevel(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:  tempDir,
		Outputs:  []io.Writer{buf},
		LogLevel: INFO, // This is now just the fallback
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Use atomic to safely track state between calls
	var levelSwitch atomic.Bool

	logger.SetDynamicLevelFunc(func() LogLevel {
		if levelSwitch.Load() {
			return DEBUG
		}
		return ERROR
	})

	// First call - dynamic level is ERROR
	logger.Debug("debug message 1") // Shouldn't log
	logger.Info("info message 1")   // Shouldn't log

	// Switch level
	levelSwitch.Store(true)

	// Second call - dynamic level is DEBUG
	logger.Debug("debug message 2") // Should log
	logger.Info("info message 2")   // Should log

	logger.Flush()

	output := buf.String()

	// Verify the output contains exactly what we expect
	expected := []string{
		"debug message 2",
		"info message 2",
	}
	notExpected := []string{
		"debug message 1",
		"info message 1",
	}

	for _, s := range expected {
		if !strings.Contains(output, s) {
			t.Errorf("Expected output to contain %q", s)
		}
	}

	for _, s := range notExpected {
		if strings.Contains(output, s) {
			t.Errorf("Expected output not to contain %q", s)
		}
	}
}

// this sometimes failing and needs improvement
// func TestConcurrentLogging(t *testing.T) {
// 	tempDir := t.TempDir()
// 	buf := &bytes.Buffer{}

// 	config := LoggerConfig{
// 		LogsDir:      tempDir,
// 		Outputs:      []io.Writer{buf},
// 		BufferSize:   1000,
// 		AsyncWorkers: 5,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	messages := 1000

// 	// Concurrent logging from multiple goroutines
// 	for i := 0; i < messages; i++ {
// 		wg.Add(1)
// 		go func(num int) {
// 			defer wg.Done()
// 			logger.Info(fmt.Sprintf("concurrent message %d", num))
// 		}(i)
// 	}

// 	wg.Wait()
// 	logger.Flush()

// 	// Verify all messages were logged
// 	output := buf.String()
// 	missing := 0
// 	for i := 0; i < messages; i++ {
// 		if !strings.Contains(output, fmt.Sprintf("concurrent message %d", i)) {
// 			missing++
// 		}
// 	}

// 	if missing > 0 {
// 		t.Errorf("Missing %d concurrent messages in output", missing)
// 	}
// }

// func TestCallerInfo(t *testing.T) {
// 	tempDir := t.TempDir()
// 	buf := &bytes.Buffer{}

// 	config := LoggerConfig{
// 		LogsDir:      tempDir,
// 		Outputs:      []io.Writer{buf},
// 		EnableCaller: true,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	// This function will appear in the caller info
// 	logTestMessage := func() {
// 		logger.Info("test message with caller")
// 	}

// 	logTestMessage()
// 	logger.Flush()

// 	output := buf.String()
// 	if !strings.Contains(output, "logger_test.go") {
// 		t.Error("Caller info not found in log output")
// 	}
// 	if !strings.Contains(output, "logTestMessage") {
// 		t.Error("Function name not found in caller info")
// 	}
// }

// func TestFatalLogging(t *testing.T) {
// 	tempDir := t.TempDir()
// 	buf := &bytes.Buffer{}

// 	config := LoggerConfig{
// 		LogsDir: tempDir,
// 		Outputs: []io.Writer{buf},
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}

// 	// Use a fake exit function to test fatal behavior
// 	exitCalled := false
// 	origExit := osExit
// 	defer func() { osExit = origExit }()
// 	osExit = func(code int) { exitCalled = true }

// 	logger.Fatal("fatal message")

// 	if !exitCalled {
// 		t.Error("Fatal log did not trigger exit")
// 	}

// 	if !strings.Contains(buf.String(), "fatal message") {
// 		t.Error("Fatal message not logged")
// 	}
// }

// func TestFileRotationSignal(t *testing.T) {
// 	tempDir := t.TempDir()

// 	config := LoggerConfig{
// 		LogsDir:     tempDir,
// 		Filename:    "rotation_signal_test",
// 		MaxBytes:    10, // Very small to ensure rotation
// 		BackupCount: 1,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	// Write enough data to trigger rotation
// 	for i := 0; i < 100; i++ {
// 		logger.Info("This is a test message to fill up the log file")
// 	}
// 	logger.Flush()

// 	// Manually trigger rotation
// 	logger.rotateChan <- struct{}{}

// 	// Wait for rotation to complete
// 	time.Sleep(100 * time.Millisecond)

// 	// Check for backup files
// 	pattern := filepath.Join(tempDir, "rotation_signal_test_*.log")
// 	backups, err := filepath.Glob(pattern)
// 	if err != nil {
// 		t.Fatalf("Failed to find backup files: %v", err)
// 	}

// 	if len(backups) != 1 {
// 		t.Errorf("Expected 1 backup file after rotation, got %d", len(backups))
// 	}
// }

// func TestRateLimiting(t *testing.T) {
// 	tempDir := t.TempDir()
// 	buf := &bytes.Buffer{}

// 	config := LoggerConfig{
// 		LogsDir:    tempDir,
// 		Outputs:    []io.Writer{buf},
// 		MaxLogRate: 10, // 10 logs per second
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	// Log more messages than the rate limit
// 	for i := 0; i < 20; i++ {
// 		logger.Info(fmt.Sprintf("message %d", i))
// 	}

// 	logger.Flush()

// 	// Count the number of messages that got through
// 	output := buf.String()
// 	count := strings.Count(output, "message")

// 	if count > 12 { // Allow some slight overflow
// 		t.Errorf("Expected <=12 messages due to rate limiting, got %d", count)
// 	}
// }
