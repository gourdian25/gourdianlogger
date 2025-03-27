package main

import (
	"bytes"
	"os"
	"time"

	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Example 1: Simple logger with default configuration
	simpleLoggerExample()

	// Example 2: Custom configured logger
	customLoggerExample()

	// Example 3: Async logger with rotation
	asyncLoggerExample()

	// Example 4: Logger with JSON configuration
	jsonConfigLoggerExample()

	// Simulate application activity
	simulateAppActivity()

	// Demonstrate dynamic configuration changes
	dynamicConfigurationDemo()
}

func simpleLoggerExample() {
	// Create a logger with default configuration
	logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Log messages at different levels
	logger.Debug("This is a debug message")
	logger.Info("This is an info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")
	// logger.Fatal("This would exit the program") // Uncomment to see fatal behavior

	// Formatted logging
	logger.Infof("User %s logged in at %s", "john_doe", time.Now().Format(time.RFC3339))
}

func customLoggerExample() {
	// Create a custom configuration
	config := gourdianlogger.LoggerConfig{
		Filename:        "myapp",               // Base filename without extension
		MaxBytes:        5 * 1024 * 1024,       // 5MB file size limit
		BackupCount:     3,                     // Keep 3 backup files
		LogLevel:        gourdianlogger.INFO,   // Only log INFO and above
		TimestampFormat: "2006-01-02 15:04:05", // Custom timestamp format
		LogsDir:         "custom_logs",         // Custom log directory
		EnableCaller:    true,                  // Include caller information
	}

	// Create the logger
	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Add an additional output (file + stdout + this new writer)
	logger.AddOutput(os.Stderr)

	// These messages will be logged
	logger.Info("This info message will appear")
	logger.Warn("This warning will appear")
	logger.Error("This error will appear")

	// This message won't appear because we set level to INFO
	logger.Debug("This debug message won't appear")
}

func asyncLoggerExample() {
	// Create an async logger configuration
	config := gourdianlogger.LoggerConfig{
		Filename:     "async_app",
		MaxBytes:     2 * 1024 * 1024, // 2MB file size limit
		BackupCount:  5,
		LogLevel:     gourdianlogger.DEBUG,
		BufferSize:   1000, // Enable async mode with 1000 entry buffer
		AsyncWorkers: 2,    // Two worker goroutines
		ShowBanner:   true, // Show initialization banner
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Log many messages quickly (these will be processed asynchronously)
	for i := 0; i < 100; i++ {
		logger.Infof("Processing item %d", i)
	}

	// Demonstrate rotation by forcing it (normally happens when file reaches max size)
	logger.Info("This message might trigger rotation")
	logger.Info("Final message before flush")

	// Ensure all async messages are processed before exiting
	logger.Flush()
}

func jsonConfigLoggerExample() {
	// JSON configuration example
	jsonConfig := `{
		"filename": "json_config_app",
		"max_bytes": 3145728,
		"backup_count": 7,
		"log_level": "WARN",
		"timestamp_format": "2006-01-02T15:04:05.000Z07:00",
		"logs_dir": "json_logs",
		"enable_caller": false,
		"buffer_size": 500,
		"async_workers": 3,
		"show_banner": false
	}`

	// Create logger from JSON config
	logger, err := gourdianlogger.WithConfig(jsonConfig)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Test logging
	logger.Debug("This debug won't appear (level is WARN)")
	logger.Info("This info won't appear (level is WARN)")
	logger.Warn("This warning will appear")
	logger.Error("This error will appear")

	// Change log level dynamically
	logger.SetLogLevel(gourdianlogger.DEBUG)
	logger.Debug("Now debug messages will appear after level change")
}

// httpRequestLogger implements io.Writer to log HTTP requests
type httpRequestLogger struct {
	logger *gourdianlogger.Logger
}

func (h *httpRequestLogger) Write(p []byte) (n int, err error) {
	// Extract relevant information from the log message
	// In a real implementation, you would parse the log entry
	h.logger.Info("HTTP request logged:", string(p))
	return len(p), nil
}

func simulateAppActivity() {

	// Create a multi-output logger
	config := gourdianlogger.LoggerConfig{
		Filename:     "advanced_app",
		MaxBytes:     10 * 1024 * 1024, // 10MB
		BackupCount:  10,
		LogLevel:     gourdianlogger.DEBUG,
		EnableCaller: true,
		BufferSize:   2000,
		AsyncWorkers: 4,
		ShowBanner:   true,
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Add an HTTP request logger as an additional output
	httpLogger := &httpRequestLogger{logger: logger}
	logger.AddOutput(httpLogger)

	// Simulate different components of an application
	go func() {
		for i := 0; i < 20; i++ {
			logger.Debugf("[AUTH] Authentication attempt %d", i)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	go func() {
		for i := 0; i < 15; i++ {
			logger.Infof("[DB] Database query executed %d", i)
			time.Sleep(150 * time.Millisecond)
		}
	}()

	go func() {
		for i := 0; i < 10; i++ {
			if i%3 == 0 {
				logger.Warnf("[CACHE] Cache miss occurred %d", i)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	time.Sleep(3 * time.Second)
}

func dynamicConfigurationDemo() {

	// Create a multi-output logger
	config := gourdianlogger.LoggerConfig{
		Filename:     "advanced_app",
		MaxBytes:     10 * 1024 * 1024, // 10MB
		BackupCount:  10,
		LogLevel:     gourdianlogger.DEBUG,
		EnableCaller: true,
		BufferSize:   2000,
		AsyncWorkers: 4,
		ShowBanner:   true,
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Add an HTTP request logger as an additional output
	httpLogger := &httpRequestLogger{logger: logger}
	logger.AddOutput(httpLogger)

	// Create a buffer to capture logs temporarily
	var buf bytes.Buffer

	// Add the buffer as a temporary output
	logger.AddOutput(&buf)

	// Log some messages that will go to the buffer
	logger.Info("This message goes to buffer and other outputs")
	logger.Warn("Another buffered message")

	// Print buffer contents
	logger.Info("Buffer contents:")
	logger.Info(buf.String())

	// Remove the buffer output
	logger.RemoveOutput(&buf)

	// Change log level
	logger.Info("Changing log level to ERROR")
	logger.SetLogLevel(gourdianlogger.ERROR)

	logger.Debug("This debug won't appear")
	logger.Info("This info won't appear")
	logger.Error("This error will appear")

	// Change back to DEBUG
	logger.SetLogLevel(gourdianlogger.DEBUG)
	logger.Debug("Debug messages are back!")
}
