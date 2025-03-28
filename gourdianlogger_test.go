package gourdianlogger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	err := os.MkdirAll("test_logs", 0755)
	if err != nil {
		fmt.Printf("Failed to create test directory: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown: Remove test logs directory
	err = os.RemoveAll("test_logs")
	if err != nil {
		fmt.Printf("Failed to clean up test directory: %v\n", err)
	}

	os.Exit(code)
}

// TestLogFormats tests all supported log formats
func TestLogFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format LogFormat
		check  func(string) bool
	}{
		{
			name:   "Plain",
			format: FormatPlain,
			check: func(s string) bool {
				return strings.Contains(s, "[INFO]") && strings.Contains(s, "test message")
			},
		},
		{
			name:   "JSON",
			format: FormatJSON,
			check: func(s string) bool {
				var data map[string]interface{}
				return json.Unmarshal([]byte(s), &data) == nil && data["level"] == "INFO"
			},
		},
		{
			name:   "JSONPretty",
			format: FormatJSON,
			check: func(s string) bool {
				var data map[string]interface{}
				config := DefaultConfig()
				config.FormatConfig.PrettyPrint = true
				return json.Unmarshal([]byte(s), &data) == nil && data["level"] == "INFO"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := DefaultConfig()
			config.Format = tt.format
			config.Outputs = []io.Writer{&buf}
			config.LogsDir = "test_logs"
			config.EnableCaller = true

			if strings.Contains(tt.name, "Pretty") {
				config.FormatConfig.PrettyPrint = true
			}

			logger, err := NewGourdianLogger(config)
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Close()

			logger.Info("test message")

			output := buf.String()
			if !tt.check(output) {
				t.Errorf("Format validation failed for %s.\nGot: %q", tt.name, output)
			}
		})
	}
}

// TestWithConfig tests JSON config parsing
func TestWithConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		jsonConfig string
		verify     func(*Logger) error
		expectErr  bool
	}{
		{
			name: "ValidConfig",
			jsonConfig: `{
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
			}`,
			verify: func(l *Logger) error {
				if l.GetLogLevel() != WARN {
					return fmt.Errorf("expected log level WARN, got %v", l.GetLogLevel())
				}
				return nil
			},
			expectErr: false,
		},
		{
			name: "InvalidJSON",
			jsonConfig: `{
				"filename": "invalid",
				"logs_dir": "test_logs",
				"log_level": "INVALID"
			}`,
			expectErr: true,
		},
		{
			name:       "EmptyConfig",
			jsonConfig: `{}`,
			verify: func(l *Logger) error {
				if l.GetLogLevel() != DEBUG {
					return fmt.Errorf("expected default log level DEBUG, got %v", l.GetLogLevel())
				}
				return nil
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := WithConfig(tt.jsonConfig)
			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			defer logger.Close()

			if tt.verify != nil {
				if err := tt.verify(logger); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

// TestDefaultConfig verifies the default configuration
func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()

	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
	}{
		{"Filename", config.Filename, "app"},
		{"MaxBytes", config.MaxBytes, defaultMaxBytes},
		{"BackupCount", config.BackupCount, defaultBackupCount},
		{"LogLevel", config.LogLevel, DEBUG},
		{"TimestampFormat", config.TimestampFormat, defaultTimestampFormat},
		{"LogsDir", config.LogsDir, defaultLogsDir},
		{"EnableCaller", config.EnableCaller, true},
		{"BufferSize", config.BufferSize, 0},
		{"AsyncWorkers", config.AsyncWorkers, 1},
		{"Format", config.Format, FormatPlain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tt.actual)
			}
		})
	}
}

// TestNewLogger tests logger creation
func TestNewLogger(t *testing.T) {
	t.Parallel()

	t.Run("BasicCreation", func(t *testing.T) {
		t.Parallel()

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

		if logger.GetLogLevel() != DEBUG {
			t.Error("Default log level should be DEBUG")
		}
	})

	t.Run("InvalidDirectory", func(t *testing.T) {
		t.Parallel()

		_, err := NewGourdianLogger(LoggerConfig{
			Filename: "test",
			LogsDir:  "/invalid/path/that/does/not/exist",
		})
		if err == nil {
			t.Error("Expected error for invalid directory")
		}
	})

	t.Run("EmptyFilename", func(t *testing.T) {
		t.Parallel()

		logger, err := NewGourdianLogger(LoggerConfig{
			Filename: "",
			LogsDir:  "test_logs",
		})
		if err != nil {
			t.Fatalf("Failed to create logger with empty filename: %v", err)
		}
		defer logger.Close()

		if !strings.HasSuffix(logger.baseFilename, "app.log") {
			t.Errorf("Expected default filename 'app.log', got '%s'", logger.baseFilename)
		}
	})

	t.Run("WithExtension", func(t *testing.T) {
		t.Parallel()

		logger, err := NewGourdianLogger(LoggerConfig{
			Filename: "test.log",
			LogsDir:  "test_logs",
		})
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		if !strings.HasSuffix(logger.baseFilename, "test.log") {
			t.Errorf("Expected filename 'test.log', got '%s'", logger.baseFilename)
		}
	})
}

// TestLogLevels tests all log level functions
func TestLogLevels(t *testing.T) {
	t.Parallel()

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
		{"Debug", func() { logger.Debug("debug message") }, "DEBUG"},
		{"Info", func() { logger.Info("info message") }, "INFO"},
		{"Warn", func() { logger.Warn("warn message") }, "WARN"},
		{"Error", func() { logger.Error("error message") }, "ERROR"},
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

// TestFatalLogging tests the fatal log level separately since it exits
func TestFatalLogging(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		config := DefaultConfig()
		config.LogsDir = "test_logs"
		config.Outputs = []io.Writer{io.Discard}

		logger, err := NewGourdianLogger(config)
		if err != nil {
			fmt.Println(err)
			return
		}
		logger.Fatal("fatal message")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalLogging")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return // Expected behavior
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

// TestLogFormatting tests formatted log functions
func TestLogFormatting(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

	// Test backup count enforcement
	for i := 0; i < 50; i++ {
		logger.Info(strings.Repeat("b", 10))
	}

	files, err = filepath.Glob(filepath.Join("test_logs", "rotation_test_*.log"))
	if err != nil {
		t.Fatal(err)
	}

	if len(files) > config.BackupCount {
		t.Errorf("Expected max %d backup files, got %d", config.BackupCount, len(files))
	}
}

// TestConcurrentLogging tests concurrent log writes
func TestConcurrentLogging(t *testing.T) {
	t.Parallel()

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
	content, err := os.ReadFile(filepath.Join("test_logs", "concurrent_test.log"))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < count {
		t.Errorf("Expected at least %d log lines, got %d", count, len(lines))
	}
}

// TestLogLevelFiltering tests that logs are properly filtered by level
func TestLogLevelFiltering(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	t.Run("AddRemoveOutput", func(t *testing.T) {
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
	})

	t.Run("SetLogLevel", func(t *testing.T) {
		logger.SetLogLevel(ERROR)
		buf.Reset()
		logger.Warn("warning message")
		if buf.Len() > 0 {
			t.Error("Logger should not output WARN messages after level change")
		}

		logger.SetLogLevel(DEBUG)
		buf.Reset()
		logger.Debug("debug message")
		if buf.Len() == 0 {
			t.Error("Logger should output DEBUG messages after level change")
		}
	})
}

// TestCloseBehavior tests logger close functionality
func TestCloseBehavior(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "close_test"

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	// Test double close
	err = logger.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}

	err = logger.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}

	// Test logging after close
	var buf bytes.Buffer
	logger.AddOutput(&buf)
	logger.Info("test after close")
	if buf.Len() > 0 {
		t.Error("Logger should not accept logs after close")
	}
}

// TestCallerInfo tests the caller information functionality
func TestCallerInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		enableCaller bool
		check        func(string) bool
	}{
		{
			name:         "CallerEnabled",
			enableCaller: true,
			check: func(s string) bool {
				return strings.Contains(s, "logger_test.go") // This test file
			},
		},
		{
			name:         "CallerDisabled",
			enableCaller: false,
			check: func(s string) bool {
				return !strings.Contains(s, "logger_test.go")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := DefaultConfig()
			config.LogsDir = "test_logs"
			config.Outputs = []io.Writer{&buf}
			config.EnableCaller = tt.enableCaller

			logger, err := NewGourdianLogger(config)
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Close()

			logger.Info("test caller info")

			if !tt.check(buf.String()) {
				t.Errorf("Caller info test failed for %s", tt.name)
			}
		})
	}
}

// BenchmarkLogging benchmarks the performance of logging
func BenchmarkLogging(b *testing.B) {
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "benchmark"
	config.Outputs = []io.Writer{io.Discard}

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
	config.Outputs = []io.Writer{io.Discard}

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
	t.Parallel()

	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "race_test"
	config.BufferSize = 100
	config.AsyncWorkers = 2

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

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
}
