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

func TestIntegrationCustomTimestampFormat(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	customFormat := "2006-Jan-02 15:04:05"
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

	testTime := time.Now()
	logger.Info("timestamp test")
	logger.Flush()

	output := buf.String()
	expected := testTime.Format(customFormat)
	if !strings.Contains(output, expected) {
		t.Errorf("Expected timestamp format %q, got: %q", expected, output)
	}
}

func TestIntegrationCallerInformation(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Outputs:      []io.Writer{buf},
		EnableCaller: true,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("caller info test")
	logger.Flush()

	output := buf.String()
	if !strings.Contains(output, "gourdianlogger_integration_test.go") {
		t.Error("Expected caller information in log output")
	}
}

func TestIntegrationPauseResume(t *testing.T) {
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

	// Log initial message
	logger.Info("before pause")

	// Pause logging
	logger.Pause()
	if !logger.IsPaused() {
		t.Error("Logger should be paused")
	}

	// These messages should not be logged
	logger.Info("during pause 1")
	logger.Info("during pause 2")

	// Resume logging
	logger.Resume()
	if logger.IsPaused() {
		t.Error("Logger should be resumed")
	}

	logger.Info("after resume")
	logger.Flush()

	output := buf.String()

	// Verify expected messages
	if !strings.Contains(output, "before pause") {
		t.Error("Missing message before pause")
	}
	if strings.Contains(output, "during pause") {
		t.Error("Logger logged messages while paused")
	}
	if !strings.Contains(output, "after resume") {
		t.Error("Missing message after resume")
	}
}

func TestIntegrationPrettyPrintedJSON(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:     tempDir,
		Outputs:     []io.Writer{buf},
		LogFormat:   FormatJSON,
		PrettyPrint: true,
		CustomFields: map[string]interface{}{
			"service": "test-service",
		},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.InfoWithFields(map[string]interface{}{
		"user": "testuser",
	}, "pretty json test")
	logger.Flush()

	output := buf.String()

	// Verify it's pretty printed (has indentation)
	if !strings.Contains(output, "  \"") {
		t.Error("JSON output is not pretty printed")
	}

	// Verify all fields are present
	expectedFields := []string{
		`"service": "test-service"`,
		`"user": "testuser"`,
		`"message": "pretty json test"`,
		`"level": "INFO"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Missing expected field in JSON: %q", field)
		}
	}
}

func TestIntegrationOutputRemoval(t *testing.T) {
	tempDir := t.TempDir()
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir: tempDir,
		Outputs: []io.Writer{buf1, buf2},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log to both outputs
	logger.Info("message to both")
	logger.Flush()

	// Remove one output
	logger.RemoveOutput(buf1)

	// Log again
	logger.Info("message to one")
	logger.Flush()

	// Verify first output only got the first message
	output1 := buf1.String()
	if strings.Count(output1, "message") != 1 {
		t.Error("First output received wrong number of messages after removal")
	}

	// Verify second output got both messages
	output2 := buf2.String()
	if strings.Count(output2, "message") != 2 {
		t.Error("Second output missing messages")
	}
}

func TestIntegrationBufferPoolEfficiency(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:    tempDir,
		Outputs:    []io.Writer{buf},
		BufferSize: 1000,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Get initial pool stats (not exposed, so we'll just test behavior)
	initialAllocs := testing.AllocsPerRun(10, func() {
		logger.Info("test message")
	})

	// After warmup, allocations should be minimal
	allocs := testing.AllocsPerRun(100, func() {
		logger.Info("test message")
	})

	if allocs > initialAllocs*2 {
		t.Errorf("High allocation count: got %v, expected similar to initial %v", allocs, initialAllocs)
	}

	logger.Flush()
}

func TestIntegrationCustomFieldsInAllMessages(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:   tempDir,
		Outputs:   []io.Writer{buf},
		LogFormat: FormatJSON,
		CustomFields: map[string]interface{}{
			"service":  "auth",
			"instance": 1,
		},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log messages at different levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")
	logger.Flush()

	output := buf.String()
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			t.Errorf("Invalid JSON: %v", err)
			continue
		}

		// Verify custom fields exist in each message
		if data["service"] != "auth" {
			t.Error("Missing service field in log message")
		}
		if data["instance"] != float64(1) { // JSON unmarshal converts to float64
			t.Error("Missing instance field in log message")
		}
	}
}

func TestIntegrationHighConcurrencyWithRateLimiting(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	rateLimit := 50 // messages per second
	config := LoggerConfig{
		LogsDir:    tempDir,
		Outputs:    []io.Writer{buf},
		MaxLogRate: rateLimit,
		BufferSize: 1000,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	messages := 500
	start := time.Now()

	for i := 0; i < messages; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("message %d", num))
		}(i)
	}

	wg.Wait()
	logger.Flush()
	duration := time.Since(start)

	// Count the messages that got through
	output := buf.String()
	count := strings.Count(output, "message")

	// Verify rate limiting was approximately enforced
	// Allow exactly the expected rate plus a small buffer for timing variations
	expectedMax := int(float64(rateLimit)*duration.Seconds()) + 5
	if count > expectedMax {
		t.Errorf("Rate limiting failed: got %d messages in %v (max expected %d)",
			count, duration, expectedMax)
	}
}

func TestIntegrationDynamicLogLevelFunction(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:  tempDir,
		Outputs:  []io.Writer{buf},
		LogLevel: INFO, // Initial level
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Use a counter to make the level changes more predictable
	callCount := 0
	logger.SetDynamicLevelFunc(func() LogLevel {
		callCount++
		// First two log calls: DEBUG
		// Next two log calls: ERROR
		// Last two log calls: DEBUG
		if callCount <= 2 || callCount > 4 {
			return DEBUG
		}
		return ERROR
	})

	// These messages should alternate between being logged and not
	logger.Debug("debug message 1") // Should log (DEBUG >= DEBUG)
	logger.Info("info message 1")   // Should log (INFO >= DEBUG)
	logger.Debug("debug message 2") // Should NOT log (DEBUG < ERROR)
	logger.Info("info message 2")   // Should NOT log (INFO < ERROR)
	logger.Debug("debug message 3") // Should log (DEBUG >= DEBUG)
	logger.Info("info message 3")   // Should log (INFO >= DEBUG)

	logger.Flush()

	output := buf.String()

	// Verify expected messages
	expected := []string{
		"debug message 1",
		"info message 1",
		"debug message 3",
		"info message 3",
	}
	notExpected := []string{
		"debug message 2",
		"info message 2",
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

	// First log some messages without pausing to ensure basic functionality
	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("pre-message %d", i))
	}

	// Pause and verify no new messages get through
	logger.Pause()
	pauseStartCount := strings.Count(buf.String(), "message")

	// Log more messages while paused
	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("paused-message %d", i))
	}

	// Verify no new messages were logged while paused
	if newCount := strings.Count(buf.String(), "message"); newCount != pauseStartCount {
		t.Errorf("Messages were logged while paused: got %d new messages", newCount-pauseStartCount)
	}

	// Resume and verify messages flow again
	logger.Resume()
	resumeStartCount := strings.Count(buf.String(), "message")

	// Log more messages after resume
	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("resumed-message %d", i))
	}

	logger.Flush()

	// Verify new messages were logged after resume
	if newCount := strings.Count(buf.String(), "message"); newCount <= resumeStartCount {
		t.Error("No messages were logged after resume")
	}

	// Verify none of the paused messages got through
	if strings.Contains(buf.String(), "paused-message") {
		t.Error("Paused messages were logged after resume")
	}
}

// TestIntegrationGetSetLogLevel tests GetLogLevel and SetLogLevel
func TestIntegrationGetSetLogLevel(t *testing.T) {
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

	// Test initial level (default is DEBUG)
	if level := logger.GetLogLevel(); level != DEBUG {
		t.Errorf("Expected initial level DEBUG, got %v", level)
	}

	// Change level and verify
	logger.SetLogLevel(INFO)
	if level := logger.GetLogLevel(); level != INFO {
		t.Errorf("Expected level INFO after SetLogLevel, got %v", level)
	}

	// Verify logging behavior reflects the level
	logger.Debug("debug message - should not appear")
	logger.Info("info message - should appear")
	logger.Flush()

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("Debug message was logged when level was INFO")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Info message was not logged when level was INFO")
	}
}

// TestIntegrationAddOutput tests adding new outputs
func TestIntegrationAddOutput(t *testing.T) {
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

	// Log initial message to first buffer
	logger.Info("message 1")
	logger.Flush()

	// Add second output
	logger.AddOutput(buf2)

	// Log message after adding second output
	logger.Info("message 2")
	logger.Flush()

	// Verify first output got both messages
	output1 := buf1.String()
	if !strings.Contains(output1, "message 1") || !strings.Contains(output1, "message 2") {
		t.Error("First output missing messages")
	}

	// Verify second output only got the message after it was added
	output2 := buf2.String()
	if strings.Contains(output2, "message 1") {
		t.Error("Second output received message before it was added")
	}
	if !strings.Contains(output2, "message 2") {
		t.Error("Second output missing message after it was added")
	}
}

// TestIntegrationConcurrentRotation tests rotation under concurrent load
func TestIntegrationConcurrentRotation(t *testing.T) {
	tempDir := t.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "concurrent_rotate",
		MaxBytes:     100, // Small to force rotation
		BackupCount:  5,
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

	// Concurrent logging that should trigger rotations
	for i := 0; i < messages; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("concurrent message %d", num))
		}(i)
	}

	wg.Wait()
	logger.Flush()

	// Verify all messages were logged (either in main or backup files)
	var totalLines int
	files, err := filepath.Glob(filepath.Join(tempDir, "concurrent_rotate*"))
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

	// Verify we didn't exceed backup count
	backups, _ := filepath.Glob(filepath.Join(tempDir, "concurrent_rotate_*.log"))
	if len(backups) > config.BackupCount {
		t.Errorf("Expected max %d backups, got %d", config.BackupCount, len(backups))
	}
}

// TestIntegrationDynamicLevelWithRotation tests dynamic level during rotation
func TestIntegrationDynamicLevelWithRotation(t *testing.T) {
	tempDir := t.TempDir()
	buf := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:     tempDir,
		Filename:    "dynlevel_rotate",
		MaxBytes:    100, // Small to force rotation
		BackupCount: 2,
		Outputs:     []io.Writer{buf},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set up dynamic level that changes after rotation
	var rotationCount int
	logger.SetDynamicLevelFunc(func() LogLevel {
		// After first rotation, switch to ERROR level
		if rotationCount > 0 {
			return ERROR
		}
		return DEBUG
	})

	// Log messages that will trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info(fmt.Sprintf("pre-rotation message %d", i))
	}

	// Force rotation check
	logger.fileMu.Lock()
	if err := logger.rotateLogFiles(); err != nil {
		logger.fileMu.Unlock()
		t.Fatalf("Rotation failed: %v", err)
	}
	rotationCount++
	logger.fileMu.Unlock()

	// Log more messages that should be filtered
	for i := 0; i < 50; i++ {
		logger.Info(fmt.Sprintf("post-rotation message %d", i))
	}
	logger.Flush()

	// Verify pre-rotation messages are present and post-rotation are filtered
	output := buf.String()
	for i := 0; i < 50; i++ {
		msg := fmt.Sprintf("pre-rotation message %d", i)
		if !strings.Contains(output, msg) {
			t.Errorf("Missing pre-rotation message: %s", msg)
		}
	}

	for i := 0; i < 50; i++ {
		msg := fmt.Sprintf("post-rotation message %d", i)
		if strings.Contains(output, msg) {
			t.Errorf("Post-rotation message should be filtered: %s", msg)
		}
	}
}

// TestIntegrationCleanupWithCustomBackupCount tests cleanup with various backup counts
func TestIntegrationCleanupWithCustomBackupCount(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		backupCount int
	}{
		{"ZeroBackups", 0},
		{"OneBackup", 1},
		{"FiveBackups", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := LoggerConfig{
				LogsDir:     tempDir,
				Filename:    "cleanup_test_" + tt.name,
				MaxBytes:    50, // Small to force rotation
				BackupCount: tt.backupCount,
			}

			logger, err := NewGourdianLogger(config)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}

			// Create enough log data to trigger multiple rotations
			for i := 0; i < 100; i++ {
				logger.Info(fmt.Sprintf("message %d for test %s", i, tt.name))
			}
			logger.Flush()
			logger.Close()

			// Check backup files
			pattern := filepath.Join(tempDir, "cleanup_test_"+tt.name+"_*.log")
			backups, err := filepath.Glob(pattern)
			if err != nil {
				t.Fatalf("Failed to find backup files: %v", err)
			}

			if tt.backupCount == 0 {
				if len(backups) > 0 {
					t.Errorf("Expected no backups with backupCount=0, got %d", len(backups))
				}
			} else if len(backups) > tt.backupCount {
				t.Errorf("Expected max %d backups, got %d", tt.backupCount, len(backups))
			}
		})
	}
}

// TestIntegrationAddOutputConcurrently tests adding outputs concurrently
func TestIntegrationAddOutputConcurrently(t *testing.T) {
	tempDir := t.TempDir()
	config := LoggerConfig{
		LogsDir: tempDir,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	outputs := make([]*bytes.Buffer, 10)

	// Concurrently add outputs
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			buf := &bytes.Buffer{}
			outputs[idx] = buf
			logger.AddOutput(buf)
		}(i)
	}

	wg.Wait()

	// Log a message that should go to all outputs
	testMsg := "message to all outputs"
	logger.Info(testMsg)
	logger.Flush()

	// Verify all outputs received the message
	for i, buf := range outputs {
		if buf == nil {
			t.Errorf("Output %d was nil", i)
			continue
		}
		if !strings.Contains(buf.String(), testMsg) {
			t.Errorf("Output %d did not receive the message", i)
		}
	}
}

// TestIntegrationRotateWithClosedFile tests rotation when file is closed
func TestIntegrationRotateWithClosedFile(t *testing.T) {
	tempDir := t.TempDir()
	config := LoggerConfig{
		LogsDir:     tempDir,
		Filename:    "rotate_closed",
		MaxBytes:    100,
		BackupCount: 2,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Close the file manually
	if err := logger.file.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}
	logger.file = nil

	// Try to rotate
	err = logger.rotateLogFiles()
	if err == nil {
		t.Error("Expected error when rotating with closed file, got nil")
	} else if !strings.Contains(err.Error(), "log file not open") {
		t.Errorf("Expected 'log file not open' error, got: %v", err)
	}

	// Clean up
	logger.Close()
}

func TestIntegrationRotateWithFailedRename(t *testing.T) {
	tempDir := t.TempDir()
	config := LoggerConfig{
		LogsDir:        tempDir,
		Filename:       "rotate_fail",
		MaxBytes:       100, // Very small size to force rotation
		BackupCount:    2,
		EnableFallback: true,
		BufferSize:     0, // Ensure synchronous logging for test
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logPath := filepath.Join(tempDir, "rotate_fail.log")

	// First verify we can write to the log
	testMsg := "initial message " + time.Now().Format(time.RFC3339Nano)
	logger.Info(testMsg)
	logger.Flush()

	// Verify initial message was written
	initialContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read initial log file: %v", err)
	}
	if !strings.Contains(string(initialContent), testMsg) {
		t.Fatalf("Initial message not found in log")
	}

	// Fill the log file to trigger rotation with messages that will exceed 100 bytes
	longMsg := strings.Repeat("x", 50) // Each message is 50 bytes
	for i := 0; i < 10; i++ {
		logger.Info(longMsg)
	}
	logger.Flush()

	// Verify file is large enough to trigger rotation
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}
	t.Logf("Current log file size: %d bytes", fileInfo.Size())

	// Simulate rename failure by making the directory read-only
	if err := os.Chmod(tempDir, 0555); err != nil {
		t.Fatalf("Failed to make directory read-only: %v", err)
	}
	defer os.Chmod(tempDir, 0755) // Clean up

	// Manually trigger rotation
	logger.fileMu.Lock()
	rotateErr := logger.rotateLogFiles()
	logger.fileMu.Unlock()

	if rotateErr == nil {
		t.Error("Expected error when rotating with read-only dir, got nil")
	} else {
		t.Logf("Rotation failed as expected: %v", rotateErr)
	}

	// Restore permissions so we can continue testing
	if err := os.Chmod(tempDir, 0755); err != nil {
		t.Fatalf("Failed to restore directory permissions: %v", err)
	}

	// Verify we can still log after failed rotation
	finalMsg := "message after failed rotation " + time.Now().Format(time.RFC3339Nano)
	logger.Info(finalMsg)
	logger.Flush()

	// Verify the message made it to the log
	finalContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read final log file: %v", err)
	}
	if !strings.Contains(string(finalContent), finalMsg) {
		t.Errorf("Final message not found in log. Content:\n%s", string(finalContent))
	}

	// Verify the original file still exists (rotation failed)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Original log file was deleted despite failed rotation")
	}
}
