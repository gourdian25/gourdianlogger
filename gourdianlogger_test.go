package gourdianlogger

// import (
// 	"bytes"
// 	"fmt"
// 	"io"
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"sync"
// 	"testing"
// )

// // Test Constants
// const (
// 	testLogDir    = "test_logs"
// 	testLogPrefix = "test_logger"
// )

// func cleanupTestFiles(t *testing.T) {
// 	t.Helper()
// 	err := os.RemoveAll(testLogDir)
// 	if err != nil {
// 		t.Fatalf("Failed to clean up test directory: %v", err)
// 	}
// }

// func TestLogLevelString(t *testing.T) {
// 	tests := []struct {
// 		level    LogLevel
// 		expected string
// 	}{
// 		{DEBUG, "DEBUG"},
// 		{INFO, "INFO"},
// 		{WARN, "WARN"},
// 		{ERROR, "ERROR"},
// 		{FATAL, "FATAL"},
// 		{LogLevel(99), "DEBUG"}, // Unknown level defaults to DEBUG
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.expected, func(t *testing.T) {
// 			if got := tt.level.String(); got != tt.expected {
// 				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
// 			}
// 		})
// 	}
// }

// func TestParseLogLevel(t *testing.T) {
// 	tests := []struct {
// 		input    string
// 		expected LogLevel
// 		wantErr  bool
// 	}{
// 		{"debug", DEBUG, false},
// 		{"INFO", INFO, false},
// 		{"warn", WARN, false},
// 		{"warning", WARN, false},
// 		{"ERROR", ERROR, false},
// 		{"fatal", FATAL, false},
// 		{"unknown", DEBUG, true},
// 		{"", DEBUG, true},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.input, func(t *testing.T) {
// 			got, err := ParseLogLevel(tt.input)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if got != tt.expected {
// 				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.expected)
// 			}
// 		})
// 	}
// }

// func TestDefaultConfig(t *testing.T) {
// 	config := DefaultConfig()

// 	if config.Filename != "app" {
// 		t.Errorf("DefaultConfig Filename = %v, want %v", config.Filename, "app")
// 	}
// 	if config.MaxBytes != defaultMaxBytes {
// 		t.Errorf("DefaultConfig MaxBytes = %v, want %v", config.MaxBytes, defaultMaxBytes)
// 	}
// 	if config.BackupCount != defaultBackupCount {
// 		t.Errorf("DefaultConfig BackupCount = %v, want %v", config.BackupCount, defaultBackupCount)
// 	}
// 	if config.LogLevel != DEBUG {
// 		t.Errorf("DefaultConfig LogLevel = %v, want %v", config.LogLevel, DEBUG)
// 	}
// 	if config.TimestampFormat != defaultTimestampFormat {
// 		t.Errorf("DefaultConfig TimestampFormat = %v, want %v", config.TimestampFormat, defaultTimestampFormat)
// 	}
// 	if config.LogsDir != defaultLogsDir {
// 		t.Errorf("DefaultConfig LogsDir = %v, want %v", config.LogsDir, defaultLogsDir)
// 	}
// }

// func TestNewGourdianLogger(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	tests := []struct {
// 		name    string
// 		config  LoggerConfig
// 		wantErr bool
// 	}{
// 		{
// 			name: "Valid Config",
// 			config: LoggerConfig{
// 				Filename:    testLogPrefix,
// 				LogsDir:     testLogDir,
// 				BackupCount: 3,
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "Invalid Directory",
// 			config: LoggerConfig{
// 				Filename:    testLogPrefix,
// 				LogsDir:     "/invalid/path",
// 				BackupCount: 3,
// 			},
// 			wantErr: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			_, err := NewGourdianLogger(tt.config)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("NewGourdianLogger() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestLoggingMethods(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	var buf bytes.Buffer

// 	config := LoggerConfig{
// 		Filename:    testLogPrefix,
// 		LogsDir:     testLogDir,
// 		BackupCount: 1,
// 		Outputs:     []io.Writer{&buf},
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	tests := []struct {
// 		name     string
// 		logFunc  func()
// 		contains string
// 		level    LogLevel
// 	}{
// 		{
// 			name: "Debug",
// 			logFunc: func() {
// 				logger.Debug("test debug message")
// 			},
// 			contains: "test debug message",
// 			level:    DEBUG,
// 		},
// 		{
// 			name: "Info",
// 			logFunc: func() {
// 				logger.Info("test info message")
// 			},
// 			contains: "test info message",
// 			level:    INFO,
// 		},
// 		{
// 			name: "Warn",
// 			logFunc: func() {
// 				logger.Warn("test warn message")
// 			},
// 			contains: "test warn message",
// 			level:    WARN,
// 		},
// 		{
// 			name: "Error",
// 			logFunc: func() {
// 				logger.Error("test error message")
// 			},
// 			contains: "test error message",
// 			level:    ERROR,
// 		},
// 		{
// 			name: "Debugf",
// 			logFunc: func() {
// 				logger.Debugf("formatted %s", "debug")
// 			},
// 			contains: "formatted debug",
// 			level:    DEBUG,
// 		},
// 		{
// 			name: "Infof",
// 			logFunc: func() {
// 				logger.Infof("formatted %s", "info")
// 			},
// 			contains: "formatted info",
// 			level:    INFO,
// 		},
// 		{
// 			name: "Warnf",
// 			logFunc: func() {
// 				logger.Warnf("formatted %s", "warn")
// 			},
// 			contains: "formatted warn",
// 			level:    WARN,
// 		},
// 		{
// 			name: "Errorf",
// 			logFunc: func() {
// 				logger.Errorf("formatted %s", "error")
// 			},
// 			contains: "formatted error",
// 			level:    ERROR,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			buf.Reset()
// 			tt.logFunc()
// 			output := buf.String()
// 			if !strings.Contains(output, tt.contains) {
// 				t.Errorf("Expected log to contain %q, got %q", tt.contains, output)
// 			}
// 			if !strings.Contains(output, tt.level.String()) {
// 				t.Errorf("Expected log level %q in output, got %q", tt.level.String(), output)
// 			}
// 		})
// 	}
// }

// func TestLogLevelFiltering(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	var buf bytes.Buffer

// 	config := LoggerConfig{
// 		Filename:    testLogPrefix,
// 		LogsDir:     testLogDir,
// 		BackupCount: 1,
// 		Outputs:     []io.Writer{&buf},
// 		LogLevel:    WARN, // Only WARN and above should be logged
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	logger.Debug("debug message")
// 	logger.Info("info message")
// 	logger.Warn("warn message")
// 	logger.Error("error message")

// 	output := buf.String()

// 	if strings.Contains(output, "debug message") {
// 		t.Error("DEBUG message was not filtered out")
// 	}
// 	if strings.Contains(output, "info message") {
// 		t.Error("INFO message was not filtered out")
// 	}
// 	if !strings.Contains(output, "warn message") {
// 		t.Error("WARN message was filtered out")
// 	}
// 	if !strings.Contains(output, "error message") {
// 		t.Error("ERROR message was filtered out")
// 	}
// }

// func TestLogRotation(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	config := LoggerConfig{
// 		Filename:    testLogPrefix,
// 		LogsDir:     testLogDir,
// 		MaxBytes:    100, // Very small size to force rotation
// 		BackupCount: 2,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	for i := 0; i < 50; i++ {
// 		logger.Info(strings.Repeat("a", 10))
// 	}

// 	logger.mu.Lock()
// 	err = logger.rotateLogFiles()
// 	logger.mu.Unlock()
// 	if err != nil {
// 		t.Fatalf("Rotation failed: %v", err)
// 	}

// 	backupFiles, err := filepath.Glob(filepath.Join(testLogDir, testLogPrefix+"_*.log"))
// 	if err != nil {
// 		t.Fatalf("Failed to list backup files: %v", err)
// 	}

// 	if len(backupFiles) == 0 {
// 		t.Error("No backup files were created")
// 	}

// 	for i := 0; i < 10; i++ {
// 		logger.Info(strings.Repeat("b", 10))
// 	}

// 	logger.mu.Lock()
// 	err = logger.rotateLogFiles()
// 	logger.mu.Unlock()
// 	if err != nil {
// 		t.Fatalf("Second rotation failed: %v", err)
// 	}

// 	backupFiles, err = filepath.Glob(filepath.Join(testLogDir, testLogPrefix+"_*.log"))
// 	if err != nil {
// 		t.Fatalf("Failed to list backup files: %v", err)
// 	}

// 	if len(backupFiles) > config.BackupCount {
// 		t.Errorf("Expected max %d backup files, got %d", config.BackupCount, len(backupFiles))
// 	}
// }

// func TestAsyncLogging(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	var buf bytes.Buffer

// 	config := LoggerConfig{
// 		Filename:     testLogPrefix,
// 		LogsDir:      testLogDir,
// 		BackupCount:  1,
// 		Outputs:      []io.Writer{&buf},
// 		BufferSize:   100,
// 		AsyncWorkers: 2,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}

// 	var wg sync.WaitGroup
// 	for i := 0; i < 100; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			logger.Info(fmt.Sprintf("message %d", i))
// 		}(i)
// 	}

// 	wg.Wait()
// 	logger.Flush()
// 	logger.Close()

// 	output := buf.String()
// 	for i := 0; i < 100; i++ {
// 		if !strings.Contains(output, fmt.Sprintf("message %d", i)) {
// 			t.Errorf("Missing message %d in output", i)
// 		}
// 	}
// }

// func TestClose(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	config := LoggerConfig{
// 		Filename:    testLogPrefix,
// 		LogsDir:     testLogDir,
// 		BackupCount: 1,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}

// 	logger.Info("test message")
// 	if err := logger.Close(); err != nil {
// 		t.Errorf("Close() error = %v", err)
// 	}

// 	if !logger.IsClosed() {
// 		t.Error("Logger should be closed")
// 	}

// 	logger.Info("should not be logged")
// }

// func TestWithConfig(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	jsonConfig := `{
// 		"filename": "json_config_test",
// 		"logs_dir": "test_logs",
// 		"max_bytes": 1024,
// 		"backup_count": 3,
// 		"log_level": "WARN",
// 		"timestamp_format": "2006-01-02",
// 		"enable_caller": false,
// 		"buffer_size": 50,
// 		"async_workers": 2
// 	}`

// 	logger, err := WithConfig(jsonConfig)
// 	if err != nil {
// 		t.Fatalf("WithConfig() error = %v", err)
// 	}
// 	defer logger.Close()

// 	if logger.GetLogLevel() != WARN {
// 		t.Errorf("Expected log level WARN, got %v", logger.GetLogLevel())
// 	}

// 	logPath := filepath.Join(testLogDir, "json_config_test.log")
// 	if _, err := os.Stat(logPath); os.IsNotExist(err) {
// 		t.Errorf("Log file was not created at %s", logPath)
// 	}
// }

// func TestAddRemoveOutput(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	var buf1, buf2 bytes.Buffer

// 	config := LoggerConfig{
// 		Filename:    testLogPrefix,
// 		LogsDir:     testLogDir,
// 		BackupCount: 1,
// 		Outputs:     []io.Writer{&buf1},
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	logger.AddOutput(&buf2)
// 	msg := "test add/remove output"
// 	logger.Info(msg)

// 	if !strings.Contains(buf1.String(), msg) {
// 		t.Error("Original output didn't receive message")
// 	}
// 	if !strings.Contains(buf2.String(), msg) {
// 		t.Error("New output didn't receive message")
// 	}

// 	logger.RemoveOutput(&buf1)
// 	buf1.Reset()
// 	buf2.Reset()

// 	msg2 := "after remove"
// 	logger.Info(msg2)

// 	if strings.Contains(buf1.String(), msg2) {
// 		t.Error("Removed output still received message")
// 	}
// 	if !strings.Contains(buf2.String(), msg2) {
// 		t.Error("Remaining output didn't receive message")
// 	}
// }

// func TestCallerInfo(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	var buf bytes.Buffer

// 	config := LoggerConfig{
// 		Filename:     testLogPrefix,
// 		LogsDir:      testLogDir,
// 		BackupCount:  1,
// 		Outputs:      []io.Writer{&buf},
// 		EnableCaller: true,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	logger.Info("test caller info")
// 	output := buf.String()
// 	if !strings.Contains(output, "gourdianlogger_test.go") {
// 		t.Error("Expected caller info in log output")
// 	}
// }

// func TestFatal(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	if os.Getenv("BE_CRASHER") == "1" {
// 		config := LoggerConfig{
// 			Filename:    testLogPrefix,
// 			LogsDir:     testLogDir,
// 			BackupCount: 1,
// 		}

// 		logger, err := NewGourdianLogger(config)
// 		if err != nil {
// 			fmt.Printf("Failed to create logger: %v\n", err)
// 			os.Exit(1)
// 		}
// 		defer logger.Close()

// 		logger.Fatal("fatal error")
// 		return
// 	}

// 	cmd := os.Command(os.Args[0], "-test.run=TestFatal")
// 	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
// 	err := cmd.Run()
// 	if e, ok := err.(*os.ExitError); ok && !e.Success() {
// 		return
// 	}
// 	t.Fatalf("process ran with err %v, want exit status 1", err)
// }

// func TestConcurrentLogging(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	var buf bytes.Buffer

// 	config := LoggerConfig{
// 		Filename:     testLogPrefix,
// 		LogsDir:      testLogDir,
// 		BackupCount:  1,
// 		Outputs:      []io.Writer{&buf},
// 		BufferSize:   100,
// 		AsyncWorkers: 4,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	iterations := 100

// 	for i := 0; i < iterations; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			logger.Info(fmt.Sprintf("concurrent message %d", i))
// 		}(i)
// 	}

// 	wg.Wait()
// 	logger.Flush()

// 	output := buf.String()
// 	for i := 0; i < iterations; i++ {
// 		if !strings.Contains(output, fmt.Sprintf("concurrent message %d", i)) {
// 			t.Errorf("Missing message %d in output", i)
// 		}
// 	}
// }

// func TestLogLevelChange(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	var buf bytes.Buffer

// 	config := LoggerConfig{
// 		Filename:    testLogPrefix,
// 		LogsDir:     testLogDir,
// 		BackupCount: 1,
// 		Outputs:     []io.Writer{&buf},
// 		LogLevel:    INFO,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	logger.Debug("debug message 1")
// 	if strings.Contains(buf.String(), "debug message 1") {
// 		t.Error("DEBUG message was not filtered")
// 	}

// 	logger.SetLogLevel(DEBUG)
// 	logger.Debug("debug message 2")
// 	if !strings.Contains(buf.String(), "debug message 2") {
// 		t.Error("DEBUG message was filtered after level change")
// 	}
// }

// func TestBufferPool(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	config := LoggerConfig{
// 		Filename:    testLogPrefix,
// 		LogsDir:     testLogDir,
// 		BackupCount: 1,
// 	}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	buf := logger.bufferPool.Get().(*bytes.Buffer)
// 	if buf == nil {
// 		t.Fatal("Got nil buffer from pool")
// 	}

// 	buf.WriteString("test")
// 	logger.bufferPool.Put(buf)

// 	buf2 := logger.bufferPool.Get().(*bytes.Buffer)
// 	if buf2.Len() != 0 {
// 		t.Error("Buffer was not reset when returned to pool")
// 	}
// }

// func TestInvalidConfigs(t *testing.T) {
// 	defer cleanupTestFiles(t)

// 	tests := []struct {
// 		name    string
// 		config  LoggerConfig
// 		wantErr bool
// 	}{
// 		{
// 			name: "Negative MaxBytes",
// 			config: LoggerConfig{
// 				Filename:    testLogPrefix,
// 				LogsDir:     testLogDir,
// 				MaxBytes:    -1,
// 				BackupCount: 1,
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "Zero BackupCount",
// 			config: LoggerConfig{
// 				Filename:    testLogPrefix,
// 				LogsDir:     testLogDir,
// 				MaxBytes:    100,
// 				BackupCount: 0,
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "Empty Filename",
// 			config: LoggerConfig{
// 				Filename:    "",
// 				LogsDir:     testLogDir,
// 				MaxBytes:    100,
// 				BackupCount: 1,
// 			},
// 			wantErr: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			_, err := NewGourdianLogger(tt.config)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("NewGourdianLogger() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestLogLevels(t *testing.T) {
// 	buf := new(bytes.Buffer)
// 	config := DefaultConfig()
// 	config.Outputs = []io.Writer{buf}
// 	config.LogLevel = INFO

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	tests := []struct {
// 		name     string
// 		logFunc  func()
// 		expected string
// 	}{
// 		{"Debug", func() { logger.Debug("test") }, ""},
// 		{"Info", func() { logger.Info("test") }, "INFO"},
// 		{"Warn", func() { logger.Warn("test") }, "WARN"},
// 		{"Error", func() { logger.Error("test") }, "ERROR"},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			buf.Reset()
// 			tt.logFunc()
// 			if tt.expected != "" && !strings.Contains(buf.String(), tt.expected) {
// 				t.Errorf("Expected log to contain %q, got %q", tt.expected, buf.String())
// 			} else if tt.expected == "" && buf.Len() > 0 {
// 				t.Errorf("Expected no output, got %q", buf.String())
// 			}
// 		})
// 	}
// }
