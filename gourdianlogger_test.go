package gourdianlogger

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// TestTimeBasedRotation tests time-based log rotation
func TestTimeBasedRotation(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.Filename = "time_rotation_test"
	config.LogsDir = "test_logs"
	config.RotationTime = 100 * time.Millisecond

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	// Wait for rotation to occur
	time.Sleep(150 * time.Millisecond)

	// Check for rotated file
	files, err := filepath.Glob(filepath.Join("test_logs", "time_rotation_test_*.log"))
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(files), 1, "Expected at least one rotated log file")
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
	require.NoError(t, err)
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
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	assert.GreaterOrEqual(t, len(lines), count, "Expected at least %d log lines", count)
}

// TestRateLimiting tests log rate limiting
func TestRateLimiting(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Outputs = []io.Writer{&buf}
	config.MaxLogRate = 10 // 10 logs per second

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	// Burst of logs
	for i := 0; i < 20; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}

	lines := strings.Split(buf.String(), "\n")
	assert.LessOrEqual(t, len(lines), config.MaxLogRate+1, "Expected logs to be rate limited")
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

// TestEnvironmentOverrides tests environment variable overrides
func TestEnvironmentOverrides(t *testing.T) {
	t.Parallel()

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

// TestLogRotation tests log rotation functionality
func TestLogRotation(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.Filename = "rotation_test"
	config.LogsDir = "test_logs"
	config.MaxBytes = 100 // Small size to trigger rotation quickly
	config.BackupCount = 2
	config.CompressBackups = true

	logger, err := NewGourdianLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	// Write enough logs to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info(strings.Repeat("a", 10))
	}

	// Force rotation
	logger.mu.Lock()
	err = logger.rotateLogFiles()
	logger.mu.Unlock()
	require.NoError(t, err)

	// Check backup files
	files, err := filepath.Glob(filepath.Join("test_logs", "rotation_test_*.log*"))
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(files), 1, "Expected at least one rotated log file")

	// Verify compression
	for _, f := range files {
		if strings.HasSuffix(f, ".gz") {
			file, err := os.Open(f)
			require.NoError(t, err, "Failed to open gzipped file")
			defer file.Close()

			_, err = gzip.NewReader(file)
			assert.NoError(t, err, "Failed to read gzipped file")
		}
	}

	// Test backup count enforcement
	for i := 0; i < 50; i++ {
		logger.Info(strings.Repeat("b", 10))
	}

	files, err = filepath.Glob(filepath.Join("test_logs", "rotation_test_*.log*"))
	require.NoError(t, err)

	assert.LessOrEqual(t, len(files), config.BackupCount+1, "Expected max %d backup files", config.BackupCount)
}
