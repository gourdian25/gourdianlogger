package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Define command-line flags
	example := flag.String("example", "basic", "Example to run (basic, json-config, async, rotation, custom-outputs, dynamic-level)")
	flag.Parse()

	switch *example {
	case "basic":
		basicExample()
	case "json-config":
		jsonConfigExample()
	case "async":
		asyncExample()
	case "rotation":
		rotationExample()
	case "custom-outputs":
		customOutputsExample()
	case "dynamic-level":
		dynamicLevelExample()
	default:
		fmt.Println("Available examples:")
		fmt.Println("  -basic: Basic logging example")
		fmt.Println("  -json-config: JSON configuration example")
		fmt.Println("  -async: Asynchronous logging example")
		fmt.Println("  -rotation: Log rotation example")
		fmt.Println("  -custom-outputs: Custom outputs example")
		fmt.Println("  -dynamic-level: Dynamic log level change example")
	}
}

func basicExample() {
	fmt.Println("=== BASIC LOGGING EXAMPLE ===")

	// Create a logger with default configuration
	logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Log messages at different levels
	logger.Debug("This is a debug message")
	logger.Info("Application started")
	logger.Warn("This is a warning")
	logger.Error("Something went wrong!")

	// Formatted logging
	logger.Infof("User %s logged in from %s", "john_doe", "192.168.1.1")
}

func jsonConfigExample() {
	fmt.Println("=== JSON CONFIGURATION EXAMPLE ===")

	// Configure logger from JSON
	configJSON := `{
		"filename": "myapp",
		"max_bytes": 5242880,
		"backup_count": 3,
		"log_level": "INFO",
		"logs_dir": "./logs",
		"enable_caller": true,
		"format": "JSON",
		"format_config": {
			"pretty_print": false,
			"custom_fields": {
				"app": "myapp",
				"version": "1.0.0"
			}
		}
	}`

	logger, err := gourdianlogger.WithConfig(configJSON)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// These messages will include the custom fields
	logger.Info("Application initialized")
	logger.Warn("Disk space running low")

	// Show the log file location
	fmt.Printf("Logs are being written to: ./logs/myapp.log\n")
}

func asyncExample() {
	fmt.Println("=== ASYNCHRONOUS LOGGING EXAMPLE ===")

	config := gourdianlogger.DefaultConfig()
	config.BufferSize = 1000   // Enable async logging with buffer
	config.AsyncWorkers = 4    // Use 4 worker goroutines
	config.EnableCaller = true // Include caller information
	config.Format = gourdianlogger.FormatPlain

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Add stdout as an additional output
	logger.AddOutput(os.Stdout)

	// Log many messages (these will be processed asynchronously)
	fmt.Println("Sending 100 log messages asynchronously...")
	for i := 0; i < 100; i++ {
		logger.Infof("Processing item %d", i)
	}

	// Ensure all logs are flushed before exiting
	logger.Flush()
	fmt.Println("All logs processed.")
}

func rotationExample() {
	fmt.Println("=== LOG ROTATION EXAMPLE ===")

	config := gourdianlogger.DefaultConfig()
	config.Filename = "rotation-demo"
	config.MaxBytes = 1024 // 1KB - small size to demonstrate rotation
	config.BackupCount = 3
	config.LogLevel = gourdianlogger.WARN // Only log WARN and above

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// This won't be logged because level is DEBUG
	logger.Debug("This debug message won't appear")

	// These will be logged
	logger.Warn("Warning: something might be wrong")
	logger.Error("Error: something is definitely wrong")

	// Force rotation by writing enough data
	fmt.Println("Filling log file to trigger rotation...")
	for i := 0; i < 100; i++ {
		logger.Warn("Filling up log file to trigger rotation...")
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Println("Check the ./logs directory for rotated files")
}

func customOutputsExample() {
	fmt.Println("=== CUSTOM OUTPUTS EXAMPLE ===")

	config := gourdianlogger.DefaultConfig()
	config.Filename = "multidest"
	config.EnableCaller = false // Disable caller info for cleaner output

	// Create a buffer to capture logs
	var logBuffer bytes.Buffer

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Add additional outputs
	logger.AddOutput(&logBuffer) // Write to buffer
	logger.AddOutput(os.Stderr)  // Write to stderr

	logger.Info("This message goes to multiple destinations")

	// Demonstrate buffer capture
	fmt.Println("\nBuffer contents:")
	fmt.Println(logBuffer.String())
}

func dynamicLevelExample() {
	fmt.Println("=== DYNAMIC LOG LEVEL EXAMPLE ===")

	logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Start with INFO level
	logger.SetLogLevel(gourdianlogger.INFO)

	fmt.Println("Setting log level to INFO")
	logger.Debug("This won't be logged") // DEBUG < INFO
	logger.Info("This will be logged")

	// Change to DEBUG level dynamically
	fmt.Println("\nChanging log level to DEBUG")
	logger.SetLogLevel(gourdianlogger.DEBUG)
	logger.Debug("Now this debug message will appear")

	// Simulate a temporary verbose logging period
	fmt.Println("\nTemporarily enabling verbose logging for 2 seconds...")
	logger.SetLogLevel(gourdianlogger.DEBUG)
	time.Sleep(2 * time.Second)
	logger.SetLogLevel(gourdianlogger.INFO)
	fmt.Println("Back to INFO level")
	logger.Debug("Debug messages hidden again")
	logger.Info("Info messages still visible")
}
