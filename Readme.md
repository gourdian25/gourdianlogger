# GourdianLogger – High-Performance Structured Logging for Go

![Go Version](https://img.shields.io/badge/Go-1.18%2B-blue)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Coverage](https://img.shields.io/badge/Coverage-75%25-yellow)](coverage.html)

**gourdianlogger** is a production-grade logging system designed for modern Go applications. Combining the flexibility of structured logging with the performance of asynchronous writes, it delivers:

- 📊 **Structured & Plain Text Logging** in both JSON and human-readable formats  
- ⚡ **High Throughput** with async buffering and worker pools (100k+ logs/sec)  
- 🔄 **Automatic Log Rotation** by size or time with gzip compression  
- 🎚️ **Dynamic Log Level Control** with runtime adjustments  
- 📡 **Multiple Outputs** including files, stdout, and custom writers  
- 🔐 **Security Features** like rate limiting and sampling  

Whether you're building microservices, CLIs, or long-running daemons, gourdianlogger provides the tools to maintain clean, actionable logs without compromising performance.

---

## 📚 Table of Contents

- [🚀 Features](#-features)
- [📦 Installation](#-installation)
- [🚀 Quick Start](#-quick-start)
- [⚙️ Configuration](#️-configuration)
- [📊 Log Levels](#-log-levels)
- [📝 Log Formats](#-log-formats)
- [🔄 Rotation & Retention](#-rotation--retention)
- [⚡ Performance](#-performance)
- [✨ Examples](#-examples)
- [✅ Best Practices](#-best-practices)
- [🧩 API Reference](#-api-reference)
- [🤝 Contributing](#-contributing)
- [🧪 Testing](#-testing)
- [📑 License](#-license)

---

## 🚀 Features

### 📊 Dual Format Logging

- **Plain Text**: Human-readable format with timestamp, level, and caller info
- **JSON**: Structured logs with embedded metadata and custom fields

```go
logger.SetFormat(gourdianlogger.FormatJSON)
```

### ⚡ Async Performance

- Configurable buffer size and worker pools
- Non-blocking writes under heavy load

```go
config.BufferSize = 1000  // 1000 log capacity
config.AsyncWorkers = 4   // 4 parallel writers
```

### 🔄 Smart Rotation

- **Size-based**: Rotate when logs exceed `MaxBytes`
- **Time-based**: Daily/weekly rotation
- **Compression**: Gzip rotated logs automatically

```go
config.MaxBytes = 50 * 1024 * 1024  // 50MB
config.RotationTime = 24 * time.Hour // Daily
config.CompressBackups = true
```

### 🎚️ Dynamic Controls

- Runtime log level changes
- Sampling to reduce log volume
- Rate limiting for noisy components

```go
logger.SetLogLevel(gourdianlogger.WARN)
config.SampleRate = 10 // Log 1 of every 10 messages
config.MaxLogRate = 100 // Max 100 logs/sec
```

### 📡 Multi-Output

- Simultaneous writing to:
  - Rotating files
  - Stdout/stderr
  - Custom writers (network, syslog, etc.)

```go
file, _ := os.Create("audit.log")
logger.AddOutput(file)
```

---

## 📦 Installation

```bash
go get github.com/gourdian25/gourdianlogger@latest
```

Requires Go 1.18+ for optimal performance.

---

## 🚀 Quick Start

### Basic Configuration

```go
package main

import (
	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Default config logs to ./logs/app.log
	logger, err := gourdianlogger.NewGourdianLoggerWithDefault()
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("Application started")
	logger.Warnf("Low disk space: %dGB remaining", 5)
}
```

### Production-Ready Setup

```go
config := gourdianlogger.LoggerConfig{
	Filename:        "myapp",
	MaxBytes:        50 * 1024 * 1024, // 50MB
	BackupCount:     7,
	LogLevel:        gourdianlogger.INFO,
	Format:          gourdianlogger.FormatJSON,
	BufferSize:      1000,
	AsyncWorkers:    4,
	CompressBackups: true,
}

logger, err := gourdianlogger.NewGourdianLogger(config)
```

---

## ⚙️ Configuration

### Core Options

| Parameter          | Description                          | Default           |
|--------------------|--------------------------------------|-------------------|
| `Filename`         | Base log filename                   | "app"             |
| `LogsDir`          | Log directory                       | "./logs"          |
| `MaxBytes`         | Max file size before rotation       | 10MB              |
| `BackupCount`      | Number of rotated logs to keep      | 5                 |
| `CompressBackups`  | Gzip rotated logs                   | false             |
| `RotationTime`     | Time-based rotation interval        | 0 (disabled)      |

### Performance

| Parameter       | Description                          | Default |
|-----------------|--------------------------------------|---------|
| `BufferSize`    | Async buffer capacity (0=sync)      | 0       |
| `AsyncWorkers`  | Parallel log writers                | 1       |
| `MaxLogRate`    | Max logs per second (0=unlimited)   | 0       |
| `SampleRate`    | Log 1 of every N messages           | 1       |

### Formatting

| Parameter          | Description                          | Default           |
|--------------------|--------------------------------------|-------------------|
| `Format`           | `FormatPlain` or `FormatJSON`       | `FormatPlain`     |
| `TimestampFormat`  | Go time format string               | RFC3339Nano       |
| `EnableCaller`     | Include file:line:function info     | true              |

---

## 📊 Log Levels

Levels in increasing severity:

| Level   | Description                          | Example Use Case               |
|---------|--------------------------------------|--------------------------------|
| `DEBUG` | Detailed diagnostic info            | `logger.Debug("Value:", x)`    |
| `INFO`  | Routine operational messages        | `logger.Info("User logged in")`|
| `WARN`  | Potentially harmful situations      | `logger.Warn("Slow query")`    |
| `ERROR` | Error conditions                    | `logger.Error(err)`            |
| `FATAL` | Severe errors triggering shutdown   | `logger.Fatal("DB unreachable")` |

Set level dynamically:

```go
if production {
	logger.SetLogLevel(gourdianlogger.WARN)
}
```

---

## 🔄 Rotation & Retention

### Size-Based Rotation

```go
config.MaxBytes = 100 * 1024 * 1024 // 100MB
config.BackupCount = 10 // Keep 10 backups
```

### Time-Based Rotation

```go
config.RotationTime = 7 * 24 * time.Hour // Weekly
```

### Retention Policy

```text
logs/
  app.log          # Current
  app_20230101.log # Rotated
  app_20230102.log.gz # Compressed
```

---

## ⚡ Performance

Benchmarks on Intel i9-13900K:

| Operation          | Throughput    | Latency       |
|--------------------|---------------|---------------|
| Sync Logging       | 85k logs/sec  | 11μs/op       |
| Async (1k buffer)  | 220k logs/sec | 4.5μs/op      |
| JSON Formatting    | 190k logs/sec | 5.2μs/op      |
| Gzip Compression   | 45 MB/sec     | -             |

---

## ✨ Examples

### Structured Logging

```go
logger.WithFields(map[string]interface{}{
	"user":    "john",
	"attempt": 3,
	"latency": 142 * time.Millisecond,
}).Error("Login failed")
```

### Error Handling

```go
logger.ErrorHandler = func(err error) {
	metrics.Increment("log_errors")
	fallbackLog.Printf("LOG FAILURE: %v", err)
}
```

### Dynamic Level Control

```go
logger.SetDynamicLevelFunc(func() gourdianlogger.LogLevel {
	if debugMode {
		return gourdianlogger.DEBUG
	}
	return gourdianlogger.INFO
})
```

---

## ✅ Best Practices

1. **Production Settings**
   ```go
   config := LoggerConfig{
	   Format:          gourdianlogger.FormatJSON,
	   BufferSize:      1000,
	   AsyncWorkers:    4,
	   MaxBytes:        100 * 1024 * 1024,
	   CompressBackups: true,
	   EnableCaller:    true,
   }
   ```

2. **Error Handling**
   ```go
   defer func() {
	   if err := logger.Close(); err != nil {
		   fmt.Fprintf(os.Stderr, "Failed to flush logs: %v", err)
	   }
   }()
   ```

3. **Security**
   ```go
   chmod 750 /var/log/myapp
   ```

---

## 🧩 API Reference

### Core Methods

```go
Debug(v ...interface{})
Info(v ...interface{})
Warn(v ...interface{})
Error(v ...interface{})
Fatal(v ...interface{})

Debugf(format string, v ...interface{})
Infof(format string, v ...interface{})
Warnf(format string, v ...interface{})
Errorf(format string, v ...interface{})
Fatalf(format string, v ...interface{})

WithFields(fields map[string]interface{}) *Entry
```

### Management

```go
SetLogLevel(level LogLevel)
AddOutput(w io.Writer)
Close() error
Flush()
```

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Submit a PR with tests

Run validation:

```bash
make test
make bench
```

---

## 📑 License

gourdianlogger is licensed under the **MIT License**.  
You are free to use, modify, distribute, and adapt the code for both personal and commercial use.

See the full license [here](./LICENSE).

---

## 👨‍💼 Maintainers

Maintained and actively developed by:

- [@gourdian25](https://github.com/gourdian25) — Creator & Core Maintainer
- [@lordofthemind](https://github.com/lordofthemind) — Performance & Benchmarking

Want to join the team? Start contributing and open a discussion!

---

## 🔒 Security Policy

We take security seriously.

- If you discover a vulnerability, please **open a private GitHub issue** or contact the maintainers directly.
- Do **not** disclose vulnerabilities in public pull requests or issues.

For all disclosures, follow responsible vulnerability reporting best practices.

---

## 📚 Documentation

Full API documentation is available on [GoDoc](https://pkg.go.dev/github.com/gourdian25/gourdianlogger).  
Includes:

- Public types and interfaces
- Usage patterns
- Token claim structures

---

Made with ❤️ by Go developers — for Go developers.  
Secure authentication shouldn't be hard. gourdianlogger makes it elegant, efficient, and production-ready.
