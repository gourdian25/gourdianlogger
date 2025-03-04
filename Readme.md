# GourdianLogger

**GourdianLogger** is a thread-safe, rotating file logger for Go applications. It provides advanced logging capabilities, including log levels, file rotation, console and file output, and customizable configurations. It is designed to be simple to use while offering powerful features for production-grade logging.

---

## Features

- **Multiple Log Levels**: Supports `DEBUG`, `INFO`, `WARN`, `ERROR`, and `FATAL` levels.
- **File Rotation**: Automatically rotates log files based on size.
- **Thread-Safe**: Safe for concurrent use across multiple goroutines.
- **Console and File Output**: Logs to both the console and a file simultaneously.
- **Customizable**:
  - Log file paths
  - Timestamp formats
  - Log levels
  - Additional outputs (e.g., external services)
- **Buffer Pooling**: Optimized for performance with reusable buffers.
- **Source Code Information**: Includes file, line number, and function name in log messages.

---

## Installation

To install `gourdianlogger`, use `go get`:

```bash
go get github.com/gourdian25/gourdianlogger@latest
```

---

## Quick Start

### Basic Usage

```go
package main

import (
	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Create a new logger with default configuration
	logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.LoggerConfig{
		Filename:    "myapp.log",
		MaxBytes:    10 * 1024 * 1024, // 10MB
		BackupCount: 5,
		LogLevel:    gourdianlogger.INFO,
	})
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Log messages at different levels
	logger.Debug("This is a debug message") // Won't be logged because level is INFO
	logger.Info("Application started")
	logger.Warn("Disk space is low")
	logger.Error("Failed to connect to database")
	// logger.Fatal("Critical error, shutting down") // Will exit the program
}
```

---

## Configuration

### LoggerConfig

The `LoggerConfig` struct allows you to customize the logger:

```go
type LoggerConfig struct {
	Filename        string      // Name of the log file (default: "gourdianlogger")
	MaxBytes        int64       // Maximum size of log file before rotation (default: 10MB)
	BackupCount     int         // Number of rotated log files to keep (default: 5)
	LogLevel        LogLevel    // Minimum log level to record (default: INFO)
	TimestampFormat string      // Custom timestamp format (default: "2006/01/02 15:04:05.000000")
	Outputs         []io.Writer // Additional outputs for logging (e.g., external services)
}
```

### Example Configuration

```go
config := gourdianlogger.LoggerConfig{
	Filename:        "myapp.log",
	MaxBytes:        10 * 1024 * 1024, // 10MB
	BackupCount:     5,
	LogLevel:        gourdianlogger.DEBUG,
	TimestampFormat: "2006-01-02 15:04:05",
	Outputs:         []io.Writer{externalServiceWriter}, // Optional: Add additional outputs
}
```

---

## Log Levels

GourdianLogger supports the following log levels, ordered by severity:

1. **DEBUG**: Detailed information for debugging.
2. **INFO**: General operational information.
3. **WARN**: Warning messages for potentially harmful situations.
4. **ERROR**: Error messages for serious problems.
5. **FATAL**: Critical errors causing program termination.

### Setting Log Levels

You can set the minimum log level at runtime:

```go
logger.SetLogLevel(gourdianlogger.DEBUG) // Log all messages
logger.SetLogLevel(gourdianlogger.ERROR) // Log only ERROR and FATAL messages
```

### Parsing Log Levels

You can parse log levels from strings:

```go
level, err := gourdianlogger.ParseLogLevel("INFO")
if err != nil {
	panic(err)
}
logger.SetLogLevel(level)
```

---

## Log Rotation

GourdianLogger automatically rotates log files when they exceed the specified size (`MaxBytes`). Old log files are renamed with a timestamp and retained up to the specified `BackupCount`.

### Example

If `MaxBytes` is set to `10 * 1024 * 1024` (10MB) and `BackupCount` is set to `5`:

- When the log file reaches 10MB, it is renamed to `myapp_20231025_150405.log`.
- A new log file is created.
- If there are more than 5 backup files, the oldest ones are deleted.

---

## Customizing Output

### Adding Additional Outputs

You can add additional outputs (e.g., external services) to the logger:

```go
logger.AddOutput(externalServiceWriter)
```

### Example: Logging to Multiple Destinations

```go
file, err := os.OpenFile("external.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
if err != nil {
	panic(err)
}
logger.AddOutput(file)
```

---

## Formatting Log Messages

Log messages are formatted as follows:

```
timestamp [LEVEL] file:line (function): message
```

Example:

```
2023/10/25 15:04:05.000000 [INFO] main.go:10 (main): Application started
```

### Customizing Timestamps

You can customize the timestamp format using the `TimestampFormat` field in `LoggerConfig`:

```go
config := gourdianlogger.LoggerConfig{
	TimestampFormat: "2006-01-02 15:04:05",
}
```

---

## API Reference

### Logger Methods

| Method            | Description                                                                 |
|-------------------|-----------------------------------------------------------------------------|
| `Debug(v ...interface{})` | Logs a message at DEBUG level.                                             |
| `Info(v ...interface{})`  | Logs a message at INFO level.                                              |
| `Warn(v ...interface{})`  | Logs a message at WARN level.                                              |
| `Error(v ...interface{})` | Logs a message at ERROR level.                                             |
| `Fatal(v ...interface{})` | Logs a message at FATAL level and terminates the program.                  |
| `Debugf(format string, v ...interface{})` | Logs a formatted message at DEBUG level.                              |
| `Infof(format string, v ...interface{})`  | Logs a formatted message at INFO level.                               |
| `Warnf(format string, v ...interface{})`  | Logs a formatted message at WARN level.                               |
| `Errorf(format string, v ...interface{})` | Logs a formatted message at ERROR level.                              |
| `Fatalf(format string, v ...interface{})` | Logs a formatted message at FATAL level and terminates the program.   |
| `SetLogLevel(level LogLevel)` | Updates the minimum log level at runtime.                             |
| `AddOutput(output io.Writer)` | Adds a new output destination for logging.                            |
| `Close() error`             | Closes the logger and its underlying file.                             |

---

## Examples

### Example 1: Basic Logging

```go
logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.LoggerConfig{
	Filename: "myapp.log",
	LogLevel: gourdianlogger.INFO,
})
if err != nil {
	panic(err)
}
defer logger.Close()

logger.Info("Application started")
logger.Warn("Disk space is low")
logger.Error("Failed to connect to database")
```

### Example 2: Custom Timestamp Format

```go
logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.LoggerConfig{
	Filename:        "myapp.log",
	TimestampFormat: "2006-01-02 15:04:05",
})
if err != nil {
	panic(err)
}
defer logger.Close()

logger.Info("Custom timestamp format")
```

### Example 3: Log Rotation

```go
logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.LoggerConfig{
	Filename:    "myapp.log",
	MaxBytes:    5 * 1024 * 1024, // 5MB
	BackupCount: 3,
})
if err != nil {
	panic(err)
}
defer logger.Close()

for i := 0; i < 100000; i++ {
	logger.Info("This is log message number:", i)
}
```

---

## License

GourdianLogger is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on [GitHub](https://github.com/gourdian25/gourdianlogger).

---
