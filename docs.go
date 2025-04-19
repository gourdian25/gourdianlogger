// Package gourdianlogger provides a high-performance, feature-rich logging solution for Go.
//
// Overview:
// GourdianLogger is designed for production systems requiring robust, configurable logging
// with minimal performance impact. It combines traditional logging with modern features
// like structured logging, sampling, and dynamic level control.
//
// Key Features:
// - Multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
// - Plain text and JSON output formats
// - Structured logging with arbitrary fields
// - Asynchronous logging with configurable buffer and workers
// - Dual rotation strategies (size-based and time-based)
// - Gzip compression of rotated logs
// - Rate limiting and log sampling
// - Caller information (file:line:function)
// - Environment variable overrides
// - Pluggable error handling
// - Thread-safe operations
//
// Getting Started:
//
// Basic example:
//
//	package main
//
//	import (
//	    "github.com/your/module/gourdianlogger"
//	)
//
//	func main() {
//	    // Create logger with default configuration
//	    logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.DefaultConfig())
//	    if err != nil {
//	        panic(err)
//	    }
//	    defer logger.Close() // Important for flushing buffers
//
//	    logger.Info("Application starting")
//	    logger.Debugf("Initialized with config: %+v", config)
//	}
//
// Configuration:
//
// The logger can be configured either programmatically or via JSON:
//
// Programmatic configuration:
//
//	config := gourdianlogger.LoggerConfig{
//	    Filename:        "myapp",
//	    LogLevel:        gourdianlogger.INFO,
//	    Format:          gourdianlogger.FormatJSON,
//	    MaxBytes:        10 * 1024 * 1024, // 10MB
//	    BackupCount:     5,
//	    CompressBackups: true,
//	}
//
// JSON configuration:
//
//	jsonCfg := `{
//	    "filename": "app",
//	    "log_level": "warn",
//	    "format": "plain",
//	    "max_bytes": 5000000,
//	    "backup_count": 3
//	}`
//	logger, err := gourdianlogger.WithConfig(jsonCfg)
//
// Log Levels:
//
// Log levels control message verbosity. Messages below the configured level are ignored.
// Level order: DEBUG < INFO < WARN < ERROR < FATAL
//
// Example level usage:
//
//	logger.SetLogLevel(gourdianlogger.WARN) // Only WARN and above
//	logger.Debug("This won't appear")      // Below current level
//	logger.Error("This will appear")       // Above current level
//
// Output Formats:
//
// Two formats are supported:
// - FormatPlain: Human-readable text (default)
// - FormatJSON: Structured JSON output
//
// Format comparison:
//
// Plain:
// 2025-04-19 10:00:00.000000 [INFO] main.go:42: Server started on port 8080
//
// JSON:
//
//	{
//	  "timestamp": "2025-04-19 10:00:00.000000",
//	  "level": "INFO",
//	  "message": "Server started on port 8080",
//	  "caller": "main.go:42"
//	}
//
// Structured Logging:
//
// Add contextual fields to log messages:
//
//	logger.InfofWithFields(map[string]interface{}{
//	    "user_id":   123,
//	    "duration":  45.2,
//	    "endpoint":  "/api/user",
//	    "success":   true,
//	}, "Request processed")
//
// Outputs (JSON):
//
//	{
//	  "timestamp": "...",
//	  "level": "INFO",
//	  "message": "Request processed",
//	  "user_id": 123,
//	  "duration": 45.2,
//	  "endpoint": "/api/user",
//	  "success": true
//	}
//
// Log Rotation:
//
// Two rotation strategies:
// 1. Size-based: Rotate when file exceeds MaxBytes
// 2. Time-based: Rotate every RotationTime duration
//
// Rotation example:
//
//	config := gourdianlogger.DefaultConfig()
//	config.MaxBytes = 50 * 1024 * 1024 // 50MB
//	config.RotationTime = 24 * time.Hour // Daily rotation
//	config.CompressBackups = true // Gzip rotated logs
//
// Creates files like:
// - app.log (current)
// - app_20250419_000000.log.gz (rotated)
// - app_20250418_000000.log.gz (rotated)
//
// Asynchronous Logging:
//
// For high-throughput systems, enable async logging:
//
//	config := gourdianlogger.DefaultConfig()
//	config.BufferSize = 1000      // Buffer capacity
//	config.AsyncWorkers = 4       // Parallel workers
//
// Important: Call logger.Flush() before shutdown to ensure all logs are written.
//
// Advanced Features:
//
// Rate Limiting:
//
//	config.MaxLogRate = 100 // Max 100 logs/second
//
// Sampling:
//
//	config.SampleRate = 10 // Log 1 in 10 messages
//
// Dynamic Level Control:
//
//	logger.SetDynamicLevelFunc(func() gourdianlogger.LogLevel {
//	    if debugMode {
//	        return gourdianlogger.DEBUG
//	    }
//	    return gourdianlogger.INFO
//	})
//
// Custom Error Handling:
//
//	logger.ErrorHandler = func(err error) {
//	    sentry.CaptureException(err)
//	    fmt.Fprintf(os.Stderr, "LOGGER ERROR: %v\n", err)
//	}
//
// Environment Overrides:
//
// Configuration can be overridden via environment variables:
// - LOG_LEVEL  (e.g. "debug", "info")
// - LOG_FORMAT ("plain" or "json")
// - LOG_DIR    (log directory path)
// - LOG_RATE   (max logs per second)
//
// Example:
//
//	os.Setenv("LOG_LEVEL", "debug")
//	os.Setenv("LOG_FORMAT", "json")
//
// Testing:
//
// For testing, you may want to:
// - Disable file output
// - Use a buffer for verification
// - Set synchronous logging
//
// Test configuration example:
//
//	func TestLogger(t *testing.T) {
//	    var buf bytes.Buffer
//	    config := gourdianlogger.DefaultConfig()
//	    config.Filename = "" // Disable file output
//	    config.Outputs = []io.Writer{&buf}
//	    config.BufferSize = 0 // Synchronous for testing
//
//	    logger, _ := gourdianlogger.NewGourdianLogger(config)
//	    logger.Info("Test message")
//	    assert.Contains(t, buf.String(), "Test message")
//	}
//
// Best Practices:
// 1. Always defer logger.Close()
// 2. For async logging, ensure proper buffer sizing
// 3. Set appropriate log levels in production
// 4. Use structured logging for machine-readable logs
// 5. Monitor log rotation in high-volume systems
// 6. Consider log sampling for high-frequency debug logs
//
// Performance Notes:
// - JSON formatting has ~10-15% overhead vs plain text
// - Async logging reduces latency but increases memory usage
// - Caller information adds minor overhead
// - Structured fields require additional allocations
package gourdianlogger
