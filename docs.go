// Package gourdianlogger provides a high-performance, feature-rich logging utility for Go applications.
//
// GourdianLogger supports the following features:
//   - Multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
//   - Structured logging with custom fields
//   - Configurable log formats (plain text or JSON)
//   - Asynchronous logging with worker pools
//   - Log file rotation by size or time
//   - Gzip compression of rotated logs
//   - Rate limiting (max logs per second)
//   - Sampling (log 1 in N messages)
//   - Caller tracing (file:line:function)
//   - Environment-based overrides
//   - Pluggable error handling and dynamic log level control
//
// GourdianLogger is designed for production use, with thread-safe operations and
// high configurability.
//
// # Basic Usage
//
//	import "github.com/your/module/gourdianlogger"
//
//	func main() {
//	    logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.DefaultConfig())
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer logger.Close()
//
//	    logger.Info("App started")
//	    logger.Warnf("Using fallback value: %s", fallback)
//	}
//
// # JSON Configuration
//
// You can configure the logger via JSON string using `WithConfig`:
//
//	jsonCfg := `{
//	    "filename": "app",
//	    "log_level": "info",
//	    "format": "json",
//	    "max_bytes": 1048576,
//	    "compress_backups": true
//	}`
//
//	logger, err := gourdianlogger.WithConfig(jsonCfg)
//
// # Environment Overrides
//
// The following environment variables can override config values:
//
//   - LOG_LEVEL     → replaces LogLevelStr
//   - LOG_FORMAT    → replaces FormatStr
//   - LOG_DIR       → replaces LogsDir
//   - LOG_RATE      → replaces MaxLogRate
//
// Example:
//
//	// Will override log level even if LoggerConfig.LogLevelStr is set to "debug"
//	os.Setenv("LOG_LEVEL", "error")
//
// # Log Levels
//
// Log levels filter what gets output. For example, setting `LogLevel = INFO` will ignore all DEBUG logs.
//
// Constants:
//   - gourdianlogger.DEBUG
//   - gourdianlogger.INFO
//   - gourdianlogger.WARN
//   - gourdianlogger.ERROR
//   - gourdianlogger.FATAL
//
// Example:
//
//	logger.SetLogLevel(gourdianlogger.WARN)
//	logger.Debug("This will be skipped")
//	logger.Warn("This will be printed")
//
// # Log Formats
//
// Log output can be formatted as plain text or JSON.
//
// Example JSON log:
//
//	{
//	  "timestamp": "2025-04-18 17:00:00.123456",
//	  "level": "INFO",
//	  "message": "Server started",
//	  "service": "api"
//	}
//
// Use `FormatConfig.PrettyPrint = true` for indented JSON.
//
// # Rotation
//
// Supports both size-based and time-based rotation.
//   - Size-based: when the log file exceeds `MaxBytes`
//   - Time-based: rotates every `RotationTime` (e.g., `time.Hour`)
//
// Rotated files follow the pattern: `filename_YYYYMMDD_HHMMSS.log`
// If `CompressBackups` is true, they are compressed as `.log.gz`
//
// # Structured Logging
//
// You can attach custom fields using `WithFields` or `*fWithFields` variants:
//
//	logger.InfofWithFields(map[string]interface{}{
//	    "user_id": uid,
//	    "event": "login",
//	}, "User authenticated")
//
// # Asynchronous Logging
//
// Enable async logging by setting `BufferSize > 0`.
// This uses a buffered channel + worker pool for non-blocking writes.
//
//	logger := gourdianlogger.LoggerConfig{
//	    BufferSize: 500,
//	    AsyncWorkers: 3,
//	}
//
// Ensure `logger.Flush()` is called before shutdown to prevent log loss.
//
// # Sampling
//
// Sampling logs 1 in N messages (useful in high-frequency systems).
// Example:
//
//	logger.SampleRate = 10 // log 1 of every 10 messages
//
// # Rate Limiting
//
// `MaxLogRate` limits logs per second. Useful for spammy components.
//
//	logger.MaxLogRate = 100 // max 100 logs/sec
//
// # Custom Error Handler
//
// Customize what happens when logging errors occur:
//
//	logger.ErrorHandler = func(err error) {
//	    metrics.Increment("log_error")
//	    fallbackLogger.Error("Logging failed: ", err)
//	}
//
// # Dynamic Log Level
//
// You can change the active log level dynamically:
//
//	logger.SetDynamicLevelFunc(func() gourdianlogger.LogLevel {
//	    if os.Getenv("DEBUG_MODE") == "true" {
//	        return gourdianlogger.DEBUG
//	    }
//	    return gourdianlogger.INFO
//	})
//
// # Shutdown
//
// Always call `logger.Close()` before your application exits to flush logs:
//
//	defer logger.Close()
//
// # Testing Tip
//
// To disable file output during unit tests:
//
//	config := gourdianlogger.DefaultConfig()
//	config.Outputs = []io.Writer{gourdianlogger.StdoutOnly()} // or custom buffer
//	config.Filename = ""
//	config.LogsDir = ""
package gourdianlogger
