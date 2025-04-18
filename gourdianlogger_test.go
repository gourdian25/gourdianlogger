package gourdianlogger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// badWriter is a writer that always fails for testing fallback behavior
type badWriter struct{}

func (w *badWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated write error")
}

func TestMain(m *testing.M) {
	// Use unique directory per test run
	dir := fmt.Sprintf("test_logs_%d", time.Now().UnixNano())
	os.Setenv("LOG_DIR", dir)

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		fmt.Printf("Failed to create test directory: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	err = os.RemoveAll(dir)
	if err != nil {
		fmt.Printf("Failed to clean up test directory: %v\n", err)
	}

	os.Exit(code)
}

// TestBasicLogging tests basic log functionality
func TestBasicLogging(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	tests := []struct {
		name     string
		logFunc  func()
		contains string
	}{
		{"Debug", func() { logger.Debug("debug message") }, "DEBUG"},
		{"Info", func() { logger.Info("info message") }, "INFO"},
		{"Warn", func() { logger.Warn("warn message") }, "WARN"},
		{"Error", func() { logger.Error("error message") }, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			assert.Contains(t, buf.String(), tt.contains)
		})
	}
}

// TestFatalLogging tests fatal log behavior
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
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
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
			name:   "PlainFormat",
			format: FormatPlain,
			check: func(s string) bool {
				return strings.Contains(s, "[INFO]") && strings.Contains(s, "test message")
			},
		},
		{
			name:   "JSONFormat",
			format: FormatJSON,
			check: func(s string) bool {
				var data map[string]interface{}
				return json.Unmarshal([]byte(s), &data) == nil && data["level"] == "INFO"
			},
		},
		{
			name:   "JSONPrettyFormat",
			format: FormatJSON,
			check: func(s string) bool {
				var data map[string]interface{}
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
			require.NoError(t, err)
			defer logger.Close()

			logger.Info("test message")
			assert.True(t, tt.check(buf.String()), "Format validation failed for %s", tt.name)
		})
	}
}

// TestLogSampling tests log sampling functionality
func TestLogSampling(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}
	config.SampleRate = 5 // 1 in 5 logs should be kept

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	// Write enough logs to get a sample
	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Greater(t, len(lines), 5, "Expected some sampled logs")
	assert.Less(t, len(lines), 50, "Expected sampling to reduce log volume")
}

// TestStructuredLogging tests structured logging with fields
func TestStructuredLogging(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format LogFormat
		check  func(string) bool
	}{
		{
			name:   "PlainWithFields",
			format: FormatPlain,
			check: func(s string) bool {
				return strings.Contains(s, "key=value") && strings.Contains(s, "test message")
			},
		},
		{
			name:   "JSONWithFields",
			format: FormatJSON,
			check: func(s string) bool {
				var data map[string]interface{}
				err := json.Unmarshal([]byte(s), &data)
				return err == nil && data["key"] == "value"
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

			logger, err := NewGourdianLogger(config)
			require.NoError(t, err)
			defer logger.Close()

			fields := map[string]interface{}{"key": "value"}
			logger.InfoWithFields(fields, "test message")

			assert.True(t, tt.check(buf.String()), "Structured logging failed for %s", tt.name)
		})
	}
}

// TestDynamicLogLevel tests dynamic log level changes
func TestDynamicLogLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	// Set initial level to WARN
	logger.SetLogLevel(WARN)
	buf.Reset()
	logger.Info("should not appear")
	assert.Empty(t, buf.String(), "Info log should not appear at WARN level")

	// Change to DEBUG level
	logger.SetLogLevel(DEBUG)
	buf.Reset()
	logger.Debug("should appear")
	assert.Contains(t, buf.String(), "should appear", "Debug log should appear at DEBUG level")

	// Test dynamic level function
	logger.SetDynamicLevelFunc(func() LogLevel {
		return ERROR
	})
	buf.Reset()
	logger.Warn("should not appear with dynamic level")
	assert.Empty(t, buf.String(), "Warn log should not appear with dynamic ERROR level")
}

// TestFallbackLogging tests fallback logging behavior
func TestFallbackLogging(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&badWriter{}}
	config.EnableFallback = true
	config.ErrorHandler = func(err error) {
		buf.WriteString("ERROR: " + err.Error())
	}

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	logger.Info("test message")

	assert.Contains(t, buf.String(), "ERROR:", "Error handler should be called")
}

// TestCustomFields tests custom fields in JSON format
func TestCustomFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.Format = FormatJSON
	config.Outputs = []io.Writer{&buf}
	config.LogsDir = "test_logs"
	config.FormatConfig.CustomFields = map[string]interface{}{
		"service": "test",
		"version": 1.0,
	}

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	logger.Info("test message")

	var data map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &data)
	require.NoError(t, err)

	assert.Equal(t, "test", data["service"], "Custom field 'service' not found")
	assert.Equal(t, 1.0, data["version"], "Custom field 'version' not found")
}

// TestCallerInfo tests caller information inclusion
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
				return strings.Contains(s, "logger_test.go")
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
			require.NoError(t, err)
			defer logger.Close()

			logger.Info("test caller info")
			assert.True(t, tt.check(buf.String()), "Caller info test failed for %s", tt.name)
		})
	}
}

// TestEnvironmentOverrides tests environment variable overrides
func TestEnvironmentOverrides(t *testing.T) {
	t.Setenv("LOG_DIR", "env_test_logs")
	t.Setenv("LOG_LEVEL", "ERROR")
	t.Setenv("LOG_FORMAT", "JSON")
	t.Setenv("LOG_RATE", "100")

	config := DefaultConfig()
	config.ApplyEnvOverrides()

	assert.Equal(t, "env_test_logs", config.LogsDir)
	assert.Equal(t, "ERROR", config.LogLevelStr)
	assert.Equal(t, "JSON", config.FormatStr)
	assert.Equal(t, 100, config.MaxLogRate)
}

// TestWithConfig tests JSON config parsing
func TestWithConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		jsonConfig string
		verify     func(*Logger) bool
		expectErr  bool
	}{
		{
			name: "ValidConfig",
			jsonConfig: `{
                "filename": "json_config_test",
                "logs_dir": "test_logs",
                "log_level": "WARN",
                "format": "JSON",
                "caller_depth": 3,
                "sample_rate": 1,
                "format_config": {
                    "pretty_print": true,
                    "custom_fields": {
                        "app": "test"
                    }
                }
            }`,
			verify: func(l *Logger) bool {
				return l.GetLogLevel() == WARN
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := WithConfig(tt.jsonConfig)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer logger.Close()

			if tt.verify != nil {
				assert.True(t, tt.verify(logger))
			}
		})
	}
}

func TestLogLevelParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected LogLevel
		hasError bool
	}{
		{"DEBUG", DEBUG, false},
		{"INFO", INFO, false},
		{"WARN", WARN, false},
		{"WARNING", WARN, false},
		{"ERROR", ERROR, false},
		{"FATAL", FATAL, false},
		{"INVALID", DEBUG, true},
		{"", DEBUG, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLogLevel(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level)
			}
		})
	}
}

func TestBufferPoolUsage(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.LogsDir = "test_logs"
	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	initial := logger.bufferPool.Get()
	logger.bufferPool.Put(initial)

	// Verify pool is being used
	logger.Info("test message")
	logger.Warn("another message")

	// Should reuse the buffer
	assert.Equal(t, initial, logger.bufferPool.Get(), "Buffer pool should reuse buffers")
}

func TestErrorHandling(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&badWriter{}}
	config.ErrorHandler = func(err error) {
		buf.WriteString("HANDLED: " + err.Error())
	}

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	logger.Info("test message")

	// Check that the error contains the core message we care about
	assert.Contains(t, buf.String(), "simulated write error")
	assert.Contains(t, buf.String(), "HANDLED:")
}

func TestDynamicLogLevelFunction(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	// Set dynamic level function that alternates between DEBUG and ERROR
	counter := 0
	logger.SetDynamicLevelFunc(func() LogLevel {
		counter++
		if counter%2 == 0 {
			return DEBUG
		}
		return ERROR
	})

	logger.Info("message 1") // Should be filtered (ERROR level)
	logger.Info("message 2") // Should appear (DEBUG level)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, 1, len(lines), "Expected only one message to pass through dynamic level filter")
}

func TestCustomTimestampFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}
	config.TimestampFormat = time.RFC3339Nano

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	logger.Info("test message")
	logLine := buf.String()

	// Try to parse the timestamp portion
	tsPart := strings.Split(logLine, " ")[0]
	_, err = time.Parse(time.RFC3339Nano, tsPart)
	assert.NoError(t, err, "Timestamp should match configured format")
}

func TestUnmarshalJSONEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		json      string
		expectErr bool
	}{
		{"Empty", "{}", false},
		{"InvalidJSON", "{", true},
		{"UnknownField", `{"unknown": "field"}`, false},
		{"InvalidLevel", `{"log_level": "INVALID"}`, true},
		{"InvalidFormat", `{"format": "INVALID"}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config LoggerConfig
			err := json.Unmarshal([]byte(tt.json), &config)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan bool)
	go func() {
		// Run test logic
		done <- true
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}

func TestAsyncLogging(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}
	config.BufferSize = 2
	config.AsyncWorkers = 1

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	// Fill up the async queue
	for i := 0; i < 5; i++ {
		logger.Info(fmt.Sprintf("async message %d", i))
	}
	logger.Flush()

	assert.Contains(t, buf.String(), "async message")
}

func TestCleanupOldBackups(t *testing.T) {
	config := DefaultConfig()
	config.LogsDir = t.TempDir()
	config.MaxBytes = 100
	config.BackupCount = 2
	config.Filename = "cleanup_test"

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	for i := 0; i < 5; i++ {
		logger.Info(strings.Repeat("log", 50))
		_ = logger.rotateLogFiles()
	}

	files, err := filepath.Glob(filepath.Join(config.LogsDir, "cleanup_test_*.log"))
	require.NoError(t, err)

	assert.LessOrEqual(t, len(files), config.BackupCount, "Should clean up old backups")
}

func TestAddRemoveOutput(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	logger.AddOutput(&buf)
	logger.Info("message to buffer")
	assert.Contains(t, buf.String(), "message to buffer")

	logger.RemoveOutput(&buf)
	buf.Reset()

	logger.Info("should not go to buffer")
	assert.Empty(t, buf.String(), "Buffer should not receive log after removal")
}

func TestCloseIdempotency(t *testing.T) {
	config := DefaultConfig()
	config.LogsDir = "test_logs"

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)

	assert.NoError(t, logger.Close())
	assert.NoError(t, logger.Close(), "Close should be idempotent")
}
func TestCompressFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.log")
	content := []byte("test log data")

	err := os.WriteFile(filePath, content, 0644)
	require.NoError(t, err)

	err = compressFile(filePath)
	require.NoError(t, err)

	// Check compressed file exists
	_, err = os.Stat(filePath + ".gz")
	assert.NoError(t, err, "Compressed file should exist")

	// Original file should be removed
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "Original file should be deleted")
}
func TestNewGourdianLoggerWithDefault(t *testing.T) {
	logger, err := NewGourdianLoggerWithDefault()
	assert.NoError(t, err)
	assert.NotNil(t, logger)

	defer logger.Close()
	logger.Info("test default config logger")
}

func TestAllStructuredLogLevels(t *testing.T) {
	var buf bytes.Buffer
	config := DefaultConfig()
	config.Outputs = []io.Writer{&buf}
	config.LogsDir = t.TempDir()
	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	fields := map[string]interface{}{"foo": "bar"}

	logger.DebugWithFields(fields, "debug")
	logger.WarnWithFields(fields, "warn")
	logger.ErrorWithFields(fields, "error")

	logger.InfofWithFields(fields, "info %d", 123)
	logger.WarnfWithFields(fields, "warn %s", "msg")
	logger.ErrorfWithFields(fields, "error %s", "failure")

	log := buf.String()
	assert.Contains(t, log, "foo=bar")
}

func TestGetCallerInfoFallback(t *testing.T) {
	logger := &Logger{}
	info := logger.getCallerInfo(9999) // Intentionally too deep in stack

	// It can be empty, but should not crash or panic
	assert.True(t, info == "" || strings.Contains(info, ":"), "Caller info fallback should be safe and formatted if returned")
}

func TestGetCallerInfoValid(t *testing.T) {
	logger := &Logger{}
	info := logger.getCallerInfo(1) // A safe depth

	assert.Contains(t, info, ".go", "Caller info should contain a Go file reference")
}
