// File: example/example.go
package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Example 1: Using default logger
	useDefaultLogger()

	// Example 2: Using custom configuration
	useCustomConfiguredLogger()

	// Example 3: Advanced usage with dynamic level and multiple outputs
	useAdvancedLogger()
}

func useDefaultLogger() {
	logger, err := gourdianlogger.NewDefaultGourdianLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		}
	}()

	logger.Info("Service initializing with default logger")
	logger.Debugf("Config loaded: %+v", "example config")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")

	// Structured logging
	logger.InfofWithFields(map[string]interface{}{
		"user_id":   123,
		"operation": "signup",
	}, "User registration completed")
}

func useCustomConfiguredLogger() {
	// Create a custom configuration
	config := gourdianlogger.LoggerConfig{
		Filename:        "custom_logs",
		LogsDir:         "custom_logs_dir",
		LogLevel:        gourdianlogger.INFO,
		BackupCount:     3,
		MaxBytes:        5 * 1024 * 1024, // 5MB
		BufferSize:      1000,
		AsyncWorkers:    2,
		EnableCaller:    true,
		TimestampFormat: "2006-01-02 15:04:05.000",
		LogFormat:       gourdianlogger.FormatJSON,
		PrettyPrint:     true,
		EnableFallback:  true,
		CustomFields: map[string]interface{}{
			"service": "auth-service",
			"version": "1.0.0",
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		log.Fatalf("Failed to initialize custom logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		}
	}()

	logger.Info("Custom configured logger initialized")
	logger.Debug("This debug message won't appear because log level is INFO")
	logger.Warnf("System load is at %d%%", 85)

	// JSON formatted log with custom fields
	logger.ErrorfWithFields(map[string]interface{}{
		"error_code": 500,
		"details":    "database connection timeout",
	}, "Failed to process request")
}

func useAdvancedLogger() {
	// Create a TCP listener for log output
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to create TCP listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("Error closing listener: %v", err)
		}
	}()

	// Start a simple TCP server to receive logs
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				log.Printf("Error closing connection: %v", err)
			}
		}()

		// Just read and print what we receive
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			println("TCP LOG RECEIVED:", string(buf[:n]))
		}
	}()

	// Create a buffer to capture logs
	var buf bytes.Buffer

	config := gourdianlogger.LoggerConfig{
		Filename:     "advanced_logs",
		LogLevel:     gourdianlogger.DEBUG,
		EnableCaller: true,
		Outputs: []io.Writer{
			&buf,      // in-memory buffer
			os.Stderr, // stderr
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		log.Fatalf("Failed to initialize advanced logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		}
	}()

	// Add TCP connection output after logger creation
	tcpConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		log.Fatalf("Failed to connect to TCP listener: %v", err)
	}
	defer func() {
		if err := tcpConn.Close(); err != nil {
			log.Printf("Error closing TCP connection: %v", err)
		}
	}()

	logger.AddOutput(tcpConn)

	// Add dynamic log level function
	logger.SetDynamicLevelFunc(func() gourdianlogger.LogLevel {
		// Example: Only log DEBUG during business hours
		if time.Now().Hour() >= 9 && time.Now().Hour() < 17 {
			return gourdianlogger.DEBUG
		}
		return gourdianlogger.INFO
	})

	// Test dynamic log level
	logger.Debug("This debug message may or may not appear depending on time of day")
	logger.Info("This info message will always appear")

	// Test pause/resume functionality
	logger.Info("This message will appear")
	logger.Pause()
	logger.Info("This message won't appear while paused")
	logger.Resume()
	logger.Info("This message appears after resume")

	// Verify buffer output
	println("Buffer contents:", buf.String())
}
