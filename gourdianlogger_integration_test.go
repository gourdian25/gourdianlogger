// File: gourdianlogger_integration_test.go

package gourdianlogger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestIntegrationLogRotationWithMultipleFiles tests the complete log rotation cycle
func TestIntegrationLogRotationWithMultipleFiles(t *testing.T) {
	tempDir := t.TempDir()

	config := LoggerConfig{
		LogsDir:     tempDir,
		Filename:    "rotation_test",
		MaxBytes:    100, // Small size to force rotation
		BackupCount: 3,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write enough data to trigger multiple rotations
	for i := 0; i < 500; i++ {
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

	// Verify the main log file exists and is not empty
	mainLog := filepath.Join(tempDir, "rotation_test.log")
	stat, err := os.Stat(mainLog)
	if err != nil {
		t.Fatalf("Main log file not found: %v", err)
	}
	if stat.Size() == 0 {
		t.Error("Main log file is empty")
	}
}

// TestIntegrationConcurrentLoggingWithRotation tests concurrent logging with rotation
func TestIntegrationConcurrentLoggingWithRotation(t *testing.T) {
	tempDir := t.TempDir()

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "concurrent_rotation",
		MaxBytes:     200, // Small size to force rotation
		BackupCount:  2,
		BufferSize:   1000,
		AsyncWorkers: 5,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	messages := 1000

	// Concurrent logging from multiple goroutines
	for i := 0; i < messages; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("concurrent message %d", num))
		}(i)
	}

	wg.Wait()
	logger.Flush()

	// Check that all messages were logged either in main or backup files
	var totalLines int
	files, err := filepath.Glob(filepath.Join(tempDir, "concurrent_rotation*"))
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", file, err)
			continue
		}
		totalLines += strings.Count(string(content), "concurrent message")
	}

	if totalLines < messages {
		t.Errorf("Expected at least %d messages, found %d", messages, totalLines)
	}
}

// TestIntegrationMultipleOutputsWithFailure tests logging to multiple outputs with one failing
func TestIntegrationMultipleOutputsWithFailure(t *testing.T) {
	tempDir := t.TempDir()
	goodBuf := &bytes.Buffer{}
	badBuf := &failingWriter{}
	errorBuf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:        tempDir,
		Outputs:        []io.Writer{goodBuf, badBuf},
		EnableFallback: true,
		ErrorHandler: func(err error) {
			fmt.Fprintf(errorBuf, "ERROR: %v", err)
		},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log messages
	for i := 0; i < 10; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}
	logger.Flush()

	// Verify good output received all messages
	goodOutput := goodBuf.String()
	for i := 0; i < 10; i++ {
		if !strings.Contains(goodOutput, fmt.Sprintf("message %d", i)) {
			t.Errorf("Good output missing message %d", i)
		}
	}

	// Verify error handler was called
	if errorBuf.String() == "" {
		t.Error("Error handler was not called for failing writer")
	}

	// Verify fallback to stderr (which we can't easily capture in test)
}

// TestIntegrationDynamicLevelChange tests changing log level while logging
func TestIntegrationDynamicLevelChange(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:  tempDir,
		Outputs:  []io.Writer{buf},
		LogLevel: INFO,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Start with INFO level
	logger.Debug("debug message 1 - should not appear")
	logger.Info("info message 1 - should appear")

	// Change to DEBUG level
	logger.SetLogLevel(DEBUG)
	logger.Debug("debug message 2 - should appear")
	logger.Info("info message 2 - should appear")

	// Change to ERROR level
	logger.SetLogLevel(ERROR)
	logger.Info("info message 3 - should not appear")
	logger.Error("error message 1 - should appear")

	logger.Flush()

	output := buf.String()

	// Verify expected messages
	expected := []string{
		"info message 1",
		"debug message 2",
		"info message 2",
		"error message 1",
	}
	notExpected := []string{
		"debug message 1",
		"info message 3",
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

// TestIntegrationMultipleLoggers tests multiple independent loggers
func TestIntegrationMultipleLoggers(t *testing.T) {
	tempDir := t.TempDir()

	// Create two loggers with different configurations
	config1 := LoggerConfig{
		LogsDir:  tempDir,
		Filename: "logger1",
		LogLevel: INFO,
	}

	config2 := LoggerConfig{
		LogsDir:  tempDir,
		Filename: "logger2",
		LogLevel: DEBUG,
	}

	logger1, err := NewGourdianLogger(config1)
	if err != nil {
		t.Fatalf("Failed to create logger1: %v", err)
	}
	defer logger1.Close()

	logger2, err := NewGourdianLogger(config2)
	if err != nil {
		t.Fatalf("Failed to create logger2: %v", err)
	}
	defer logger2.Close()

	// Log to both loggers
	logger1.Debug("logger1 debug - should not appear")
	logger1.Info("logger1 info - should appear")
	logger2.Debug("logger2 debug - should appear")
	logger2.Info("logger2 info - should appear")

	logger1.Flush()
	logger2.Flush()

	// Verify logger1 output
	content1, err := os.ReadFile(filepath.Join(tempDir, "logger1.log"))
	if err != nil {
		t.Fatalf("Failed to read logger1 output: %v", err)
	}
	output1 := string(content1)

	if strings.Contains(output1, "logger1 debug") {
		t.Error("Logger1 logged DEBUG message despite INFO level")
	}
	if !strings.Contains(output1, "logger1 info") {
		t.Error("Logger1 didn't log INFO message")
	}

	// Verify logger2 output
	content2, err := os.ReadFile(filepath.Join(tempDir, "logger2.log"))
	if err != nil {
		t.Fatalf("Failed to read logger2 output: %v", err)
	}
	output2 := string(content2)

	if !strings.Contains(output2, "logger2 debug") {
		t.Error("Logger2 didn't log DEBUG message")
	}
	if !strings.Contains(output2, "logger2 info") {
		t.Error("Logger2 didn't log INFO message")
	}
}

// TestIntegrationCloseWithPendingMessages tests closing with pending async messages
func TestIntegrationCloseWithPendingMessages(t *testing.T) {
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

	// Fill the async queue
	for i := 0; i < 150; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}

	// Close without explicit Flush - should still process pending messages
	err = logger.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify all messages were written
	output := buf.String()
	missing := 0
	for i := 0; i < 150; i++ {
		if !strings.Contains(output, fmt.Sprintf("message %d", i)) {
			missing++
		}
	}

	if missing > 0 {
		t.Errorf("Missing %d messages in output after Close()", missing)
	}
}

// TestIntegrationLogFormatSwitchingWithFields tests switching formats with fields
func TestIntegrationLogFormatSwitchingWithFields(t *testing.T) {
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

	fields := map[string]interface{}{
		"user": "testuser",
		"id":   123,
	}

	// Test plain format
	logger.format = FormatPlain
	logger.InfoWithFields(fields, "plain message")
	plainOutput := buf.String()
	buf.Reset()

	if !strings.Contains(plainOutput, "[INFO] plain message") {
		t.Error("Plain format not working as expected")
	}
	if !strings.Contains(plainOutput, "user=testuser") {
		t.Error("Plain format missing field")
	}

	// Test JSON format
	logger.format = FormatJSON
	logger.InfoWithFields(fields, "json message")
	jsonOutput := buf.String()

	if !strings.Contains(jsonOutput, `"level":"INFO"`) ||
		!strings.Contains(jsonOutput, `"message":"json message"`) ||
		!strings.Contains(jsonOutput, `"user":"testuser"`) {
		t.Error("JSON format not working as expected")
	}
}

// TestIntegrationFileRotationWithCustomBackupCount tests rotation with custom backup count
func TestIntegrationFileRotationWithCustomBackupCount(t *testing.T) {
	tempDir := t.TempDir()
	backupCount := 5

	config := LoggerConfig{
		LogsDir:     tempDir,
		Filename:    "backup_count_test",
		MaxBytes:    100, // Small size to force rotation
		BackupCount: backupCount,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write enough data to trigger multiple rotations
	for i := 0; i < 1000; i++ {
		logger.Info("This is a test message that should be long enough to trigger rotation")
	}
	logger.Flush()

	// Check backup files
	pattern := filepath.Join(tempDir, "backup_count_test_*.log")
	backups, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Failed to find backup files: %v", err)
	}

	if len(backups) > backupCount {
		t.Errorf("Expected max %d backups, got %d", backupCount, len(backups))
	}

	// Verify the oldest backups were deleted
	if len(backups) == backupCount {
		// Get all possible backups (including those that might have been deleted)
		allFiles, _ := filepath.Glob(filepath.Join(tempDir, "backup_count_test*"))
		if len(allFiles) > backupCount+1 { // +1 for main log file
			t.Errorf("Found %d files when expecting max %d", len(allFiles), backupCount+1)
		}
	}
}

// TestIntegrationJSONFormatWithCustomFields tests JSON formatting with custom fields
func TestIntegrationJSONFormatWithCustomFields(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:   tempDir,
		Outputs:   []io.Writer{buf},
		LogFormat: FormatJSON,
		CustomFields: map[string]interface{}{
			"service": "auth-service",
			"version": "1.2.3",
		},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log with additional fields
	fields := map[string]interface{}{
		"user_id": 12345,
		"action":  "login",
	}
	logger.InfoWithFields(fields, "User logged in")

	logger.Flush()

	output := buf.String()

	// Verify all fields are present
	expectedFields := []string{
		`"service":"auth-service"`,
		`"version":"1.2.3"`,
		`"user_id":12345`,
		`"action":"login"`,
		`"message":"User logged in"`,
		`"level":"INFO"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected field %q in output", field)
		}
	}

	// Verify valid JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}
}

// TestIntegrationRateLimitingWithConcurrency tests rate limiting under concurrent load
func TestIntegrationRateLimitingWithConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:    tempDir,
		Outputs:    []io.Writer{buf},
		MaxLogRate: 100, // 100 logs per second
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	messages := 500

	// Concurrent logging that would exceed rate limit if not enforced
	for i := 0; i < messages; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("message %d", num))
		}(i)
	}
	wg.Wait()
	logger.Flush()

	// Count the number of messages that got through
	output := buf.String()
	count := strings.Count(output, "message")

	// Verify rate was approximately enforced
	expectedMax := config.MaxLogRate + 10
	if count > expectedMax {
		t.Errorf("Expected <=%d messages due to rate limiting, got %d", expectedMax, count)
	}
}

// TestIntegrationPauseResumeWithConcurrency tests pause/resume functionality under concurrent load
func TestIntegrationPauseResumeWithConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Outputs:      []io.Writer{buf},
		BufferSize:   1000,
		AsyncWorkers: 5,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	messages := 500

	// Start logging in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < messages; i++ {
			logger.Info(fmt.Sprintf("message %d", i))
		}
	}()

	// Pause and resume multiple times
	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)
		logger.Pause()
		time.Sleep(10 * time.Millisecond)
		logger.Resume()
	}

	wg.Wait()
	logger.Flush()

	// Count messages - exact count isn't predictable due to concurrency
	// but we should have some messages
	output := buf.String()
	count := strings.Count(output, "message")
	if count == 0 {
		t.Error("No messages were logged")
	}
	if count >= messages {
		t.Error("Pause didn't seem to have any effect")
	}
}

// TestIntegrationCustomTimestampFormat tests custom timestamp formatting
func TestIntegrationCustomTimestampFormat(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	customFormat := "2006-Jan-02 15:04:05.000"
	config := LoggerConfig{
		LogsDir:         tempDir,
		Outputs:         []io.Writer{buf},
		TimestampFormat: customFormat,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("test message")
	logger.Flush()

	output := buf.String()

	// Try to parse the timestamp with our custom format
	parts := strings.SplitN(output, " ", 2)
	if len(parts) < 1 {
		t.Fatal("Couldn't parse timestamp from log output")
	}

	_, err = time.Parse(customFormat, parts[0])
	if err != nil {
		t.Errorf("Timestamp %q doesn't match format %q: %v", parts[0], customFormat, err)
	}
}
