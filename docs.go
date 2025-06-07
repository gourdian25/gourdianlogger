// File: docs.go

// Package gourdianlogger provides a high-performance, feature-rich logging solution for Go.
//
// Overview:
// GourdianLogger is designed for production systems requiring robust, configurable logging
// with minimal performance impact. It combines traditional logging with modern features
// like structured logging, rate limiting, and dynamic level control.
//
// Key Features:
// - Multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
// - Plain text and JSON output formats
// - Structured logging with arbitrary fields
// - Asynchronous logging with configurable buffer and workers
// - Size-based log rotation
// - Rate limiting
// - Caller information (file:line:function)
// - Fallback mechanisms
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
//	    logger, err := gourdianlogger.NewDefaultGourdianLogger()
//	    if err != nil {
//	        panic(err)
//	    }
//	    defer logger.Close() // Important for flushing buffers
//
//	    logger.Info("Application starting")
//	    logger.Debug("Initialization complete")
//	}
//
// Configuration:
//
// The logger can be configured either programmatically:
//
//	config := gourdianlogger.LoggerConfig{
//	    Filename:        "myapp",
//	    LogLevel:        gourdianlogger.INFO,
//	    LogFormat:       gourdianlogger.FormatJSON,
//	    MaxBytes:        10 * 1024 * 1024, // 10MB
//	    BackupCount:     5,
//	    BufferSize:      1000,             // Async buffer size
//	    AsyncWorkers:    4,                // Number of async workers
//	    MaxLogRate:      1000,             // Max logs per second
//	    EnableCaller:    true,             // Include caller info
//	    EnableFallback:  true,             // Fallback to stderr on errors
//	}
//	logger, err := gourdianlogger.NewGourdianLogger(config)
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
// Dynamic Level Control:
//
//	logger.SetDynamicLevelFunc(func() gourdianlogger.LogLevel {
//	    if debugMode {
//	        return gourdianlogger.DEBUG
//	    }
//	    return gourdianlogger.INFO
//	})
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
// 2025-04-19 10:00:00.000000 [INFO] main.go:42:main Server started on port 8080
//
// JSON:
//
//	{
//	  "timestamp": "2025-04-19 10:00:00.000000",
//	  "level": "INFO",
//	  "message": "Server started on port 8080",
//	  "caller": "main.go:42:main"
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
//	  "endpoint": "/api/user"
//	}
//
// Log Rotation:
//
// Logs are rotated when they exceed MaxBytes. Old logs are kept according to BackupCount.
//
// Rotation example:
//
//	config := gourdianlogger.LoggerConfig{
//	    MaxBytes:    50 * 1024 * 1024, // 50MB
//	    BackupCount: 5,                // Keep 5 backups
//	}
//
// Creates files like:
// - myapp.log (current)
// - myapp_20250419_000000.log (rotated)
// - myapp_20250418_000000.log (rotated)
//
// Asynchronous Logging:
//
// For high-throughput systems, enable async logging:
//
//	config := gourdianlogger.LoggerConfig{
//	    BufferSize:   1000, // Buffer capacity
//	    AsyncWorkers: 4,    // Parallel workers
//	}
//
// Important: Call logger.Flush() before shutdown to ensure all logs are written.
//
// Rate Limiting:
//
//	config.MaxLogRate = 100 // Max 100 logs/second
//
// Messages exceeding the rate limit are silently dropped.
//
// Error Handling:
//
// Custom error handling can be configured:
//
//	config.ErrorHandler = func(err error) {
//	    sentry.CaptureException(err)
//	    fmt.Fprintf(os.Stderr, "LOGGER ERROR: %v\n", err)
//	}
//
// Fallback Mechanism:
//
// When enabled (default), failed log writes will attempt to write to stderr:
// FALLBACK LOG: [message content]
//
// Pausing Logging:
//
// Temporarily pause logging during critical sections:
//
//	logger.Pause()
//	defer logger.Resume()
//	// Critical section
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
//	    config := gourdianlogger.LoggerConfig{
//	        Outputs:     []io.Writer{&buf},
//	        BufferSize:  0, // Synchronous for testing
//	        EnableCaller: false,
//	    }
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
// 6. Consider rate limiting for high-frequency debug logs
//
// Performance Notes:
// - JSON formatting has ~10-15% overhead vs plain text
// - Async logging reduces latency but increases memory usage
// - Caller information adds minor overhead
// - Structured fields require additional allocations
// - Rate limiting adds minimal overhead when not active
package gourdianlogger
