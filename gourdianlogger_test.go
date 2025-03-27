package gourdianlogger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestMain sets up and tears down any test dependencies
func TestMain(m *testing.M) {
	// Setup: Create a test logs directory
	os.MkdirAll("test_logs", 0755)

	// Run tests
	code := m.Run()

	// Teardown: Remove test logs directory
	os.RemoveAll("test_logs")

	os.Exit(code)
}

// TestDefaultConfig verifies the default configuration
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Filename != "app" {
		t.Errorf("Expected default filename 'app', got '%s'", config.Filename)
	}
	if config.MaxBytes != defaultMaxBytes {
		t.Errorf("Expected default max bytes %d, got %d", defaultMaxBytes, config.MaxBytes)
	}
	if config.LogLevel != DEBUG {
		t.Error("Expected default log level DEBUG")
	}
}

// TestNewLogger tests logger creation
func TestNewLogger(t *testing.T) {
	t.Run("BasicCreation", func(t *testing.T) {
		logger, err := NewGourdianLogger(LoggerConfig{
			Filename: "test",
			LogsDir:  "test_logs",
		})
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		if logger == nil {
			t.Error("Logger should not be nil")
		}
	})

	t.Run("InvalidDirectory", func(t *testing.T) {
		_, err := NewGourdianLogger(LoggerConfig{
			Filename: "test",
			LogsDir:  "/invalid/path",
		})
		if err == nil {
			t.Error("Expected error for invalid directory")
		}
	})
}

// TestLogLevels tests all log level functions
func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	tests := []struct {
		name   string
		fn     func()
		expect string
	}{
		{"Debug", func() { logger.Debug("debug") }, "DEBUG"},
		{"Info", func() { logger.Info("info") }, "INFO"},
		{"Warn", func() { logger.Warn("warn") }, "WARN"},
		{"Error", func() { logger.Error("error") }, "ERROR"},
		{"Fatal", func() { logger.Fatal("fatal") }, "FATAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.fn()
			if !strings.Contains(buf.String(), tt.expect) {
				t.Errorf("Expected log to contain '%s', got '%s'", tt.expect, buf.String())
			}
		})
	}
}

// TestLogFormatting tests formatted log functions
func TestLogFormatting(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	tests := []struct {
		name   string
		fn     func()
		expect string
	}{
		{"Debugf", func() { logger.Debugf("debug %s", "value") }, "debug value"},
		{"Infof", func() { logger.Infof("info %d", 42) }, "info 42"},
		{"Warnf", func() { logger.Warnf("warn %.1f", 3.14) }, "warn 3.1"},
		{"Errorf", func() { logger.Errorf("error %v", os.ErrNotExist) }, "error file does not exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.fn()
			if !strings.Contains(buf.String(), tt.expect) {
				t.Errorf("Expected log to contain '%s', got '%s'", tt.expect, buf.String())
			}
		})
	}
}

// TestLogRotation tests log file rotation
func TestLogRotation(t *testing.T) {
	config := DefaultConfig()
	config.Filename = "rotation_test"
	config.LogsDir = "test_logs"
	config.MaxBytes = 100 // Small size to trigger rotation quickly
	config.BackupCount = 2

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// Write enough logs to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info(strings.Repeat("a", 10)) // Each log is ~50 bytes with timestamp, etc.
	}

	// Force rotation
	logger.mu.Lock()
	err = logger.rotateLogFiles()
	logger.mu.Unlock()
	if err != nil {
		t.Fatalf("Rotation failed: %v", err)
	}

	// Check backup files
	files, err := filepath.Glob(filepath.Join("test_logs", "rotation_test_*.log"))
	if err != nil {
		t.Fatal(err)
	}

	if len(files) < 1 {
		t.Error("Expected at least one rotated log file")
	}
}

// TestConcurrentLogging tests concurrent log writes
func TestConcurrentLogging(t *testing.T) {
	config := DefaultConfig()
	config.Filename = "concurrent_test"
	config.LogsDir = "test_logs"
	config.BufferSize = 1000
	config.AsyncWorkers = 4

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	count := 100

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			logger.Infof("Log message %d", n)
		}(i)
	}

	wg.Wait()
	logger.Flush()

	// Verify all logs were written
	content, err := ioutil.ReadFile(filepath.Join("test_logs", "concurrent_test.log"))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < count {
		t.Errorf("Expected at least %d log lines, got %d", count, len(lines))
	}
}

// TestLogFormats tests all supported log formats
func TestLogFormats(t *testing.T) {
	tests := []struct {
		name   string
		format LogFormat
		check  func(string) bool
	}{
		{"Plain", FormatPlain, func(s string) bool {
			return strings.Contains(s, "[INFO]") && strings.Contains(s, "test message")
		}},
		{"JSON", FormatJSON, func(s string) bool {
			var data map[string]interface{}
			return json.Unmarshal([]byte(s), &data) == nil && data["level"] == "INFO"
		}},
		{"GELF", FormatGELF, func(s string) bool {
			var data map[string]interface{}
			return json.Unmarshal([]byte(s), &data) == nil && data["short_message"] == "test message"
		}},
		{"CSV", FormatCSV, func(s string) bool {
			return strings.Count(s, ",") >= 2 // timestamp,level,message
		}},
		{"CEF", FormatCEF, func(s string) bool {
			return strings.HasPrefix(s, "CEF:") && strings.Contains(s, "test message")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := DefaultConfig()
			config.Format = tt.format
			config.Outputs = []io.Writer{&buf}

			logger, err := NewGourdianLogger(config)
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Close()

			logger.Info("test message")

			if !tt.check(buf.String()) {
				t.Errorf("Format validation failed for %s: %s", tt.name, buf.String())
			}
		})
	}
}

// TestWithConfig tests JSON config parsing
func TestWithConfig(t *testing.T) {
	jsonConfig := `{
		"filename": "json_config_test",
		"logs_dir": "test_logs",
		"log_level": "WARN",
		"format": "JSON",
		"format_config": {
			"pretty_print": true,
			"custom_fields": {
				"app": "test"
			}
		}
	}`

	logger, err := WithConfig(jsonConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	if logger.GetLogLevel() != WARN {
		t.Error("Expected log level WARN from config")
	}

	var buf bytes.Buffer
	logger.AddOutput(&buf)
	logger.Warn("test warning")
	logger.RemoveOutput(&buf)

	var data map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if data["app"] != "test" {
		t.Error("Expected custom field 'app' in log")
	}
}

// TestLogLevelFiltering tests that logs are properly filtered by level
func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}
	config.LogLevel = WARN

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	if strings.Contains(output, "debug") || strings.Contains(output, "info") {
		t.Error("Logger should not output messages below WARN level")
	}
	if !strings.Contains(output, "warn") || !strings.Contains(output, "error") {
		t.Error("Logger should output WARN and ERROR messages")
	}
}

// TestDynamicConfiguration tests dynamic changes to logger config
func TestDynamicConfiguration(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// Test AddOutput/RemoveOutput
	var buf2 bytes.Buffer
	logger.AddOutput(&buf2)
	logger.Info("test message")
	if buf2.Len() == 0 {
		t.Error("Second output should have received log message")
	}

	logger.RemoveOutput(&buf2)
	buf2.Reset()
	logger.Info("another message")
	if buf2.Len() > 0 {
		t.Error("Second output should no longer receive messages")
	}

	// Test SetLogLevel
	logger.SetLogLevel(ERROR)
	buf.Reset()
	logger.Warn("warning message")
	if buf.Len() > 0 {
		t.Error("Logger should not output WARN messages after level change")
	}
}

// BenchmarkLogging benchmarks the performance of logging
func BenchmarkLogging(b *testing.B) {
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "benchmark"
	config.Outputs = []io.Writer{ioutil.Discard} // Discard output for benchmarking

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark log message")
	}
}

// BenchmarkConcurrentLogging benchmarks concurrent log writes
func BenchmarkConcurrentLogging(b *testing.B) {
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "benchmark_concurrent"
	config.BufferSize = 1000
	config.AsyncWorkers = 4
	config.Outputs = []io.Writer{ioutil.Discard}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("concurrent benchmark log message")
		}
	})
}

// TestRaceConditions runs tests with race detector
func TestRaceConditions(t *testing.T) {
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "race_test"
	config.BufferSize = 100
	config.AsyncWorkers = 2

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	// Test concurrent log writes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			logger.Infof("goroutine 1: %d", i)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			logger.Errorf("goroutine 2: %d", i)
		}
	}()

	// Test concurrent configuration changes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			logger.SetLogLevel(LogLevel(i % 5))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			logger.AddOutput(&buf)
			logger.RemoveOutput(&buf)
		}
	}()

	wg.Wait()
	logger.Close()
}

func TestFatal(t *testing.T) {
	if os.Getenv("TEST_FATAL") == "1" {
		config := DefaultConfig()
		config.LogsDir = "test_logs"
		config.Outputs = []io.Writer{ioutil.Discard}

		logger, err := NewGourdianLogger(config)
		if err != nil {
			fmt.Println(err)
			return
		}
		logger.Fatal("fatal message")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatal")
	cmd.Env = append(os.Environ(), "TEST_FATAL=1")
	err := cmd.Run()

	// Correct way to check for exit status
	if exitErr, ok := err.(*exec.ExitError); ok {
		if !exitErr.Success() {
			return // Expected non-zero exit status
		}
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}
