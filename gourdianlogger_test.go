package gourdianlogger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGourdianLogger(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		logger, err := NewGourdianLogger(DefaultConfig())
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		if logger.GetLogLevel() != DEBUG {
			t.Errorf("Expected log level DEBUG, got %v", logger.GetLogLevel())
		}

		if logger.maxBytes != defaultMaxBytes {
			t.Errorf("Expected maxBytes %d, got %d", defaultMaxBytes, logger.maxBytes)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		config := LoggerConfig{
			Filename:        "test",
			MaxBytes:        1024,
			BackupCount:     3,
			LogLevel:        WARN,
			TimestampFormat: "2006",
			LogsDir:         "testlogs",
			EnableCaller:    false,
		}

		logger, err := NewGourdianLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		defer os.RemoveAll("testlogs")

		if logger.GetLogLevel() != WARN {
			t.Errorf("Expected log level WARN, got %v", logger.GetLogLevel())
		}

		if logger.maxBytes != 1024 {
			t.Errorf("Expected maxBytes 1024, got %d", logger.maxBytes)
		}

		if _, err := os.Stat("testlogs"); os.IsNotExist(err) {
			t.Error("Logs directory was not created")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		config := LoggerConfig{
			Filename:    "",
			MaxBytes:    -1,
			BackupCount: -1,
		}

		logger, err := NewGourdianLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		if logger.maxBytes <= 0 {
			t.Errorf("Expected positive maxBytes, got %d", logger.maxBytes)
		}
	})
}

func TestLogLevels(t *testing.T) {
	buf := new(bytes.Buffer)
	config := DefaultConfig()
	config.Outputs = []io.Writer{buf}
	config.LogLevel = INFO

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{"Debug", func() { logger.Debug("test") }, ""},
		{"Info", func() { logger.Info("test") }, "INFO"},
		{"Warn", func() { logger.Warn("test") }, "WARN"},
		{"Error", func() { logger.Error("test") }, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			if tt.expected != "" && !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("Expected log to contain %q, got %q", tt.expected, buf.String())
			} else if tt.expected == "" && buf.Len() > 0 {
				t.Errorf("Expected no output, got %q", buf.String())
			}
		})
	}
}

func TestLogRotation(t *testing.T) {
	dir := "test_rotate"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	config := DefaultConfig()
	config.LogsDir = dir
	config.Filename = "rotate"
	config.MaxBytes = 10 // Small size to force rotation

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write enough to trigger rotation
	for i := 0; i < 100; i++ {
		logger.Info(strings.Repeat("a", 10))
	}

	// Manually trigger rotation
	logger.mu.Lock()
	err = logger.rotateLogFiles()
	logger.mu.Unlock()
	if err != nil {
		t.Fatalf("Rotation failed: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "rotate_*.log"))
	if err != nil {
		t.Fatalf("Failed to find rotated files: %v", err)
	}

	if len(matches) == 0 {
		t.Error("No rotated files found")
	}
}

func TestClose(t *testing.T) {
	logger, err := NewGourdianLogger(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !logger.IsClosed() {
		t.Error("Logger should be closed")
	}

	// Double close should not panic
	if err := logger.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

func TestCallerInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	config := DefaultConfig()
	config.Outputs = []io.Writer{buf}
	config.EnableCaller = true

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("test")
	output := buf.String()

	if !strings.Contains(output, "gourdianlogger_test.go") {
		t.Error("Expected caller info in log output")
	}
}
