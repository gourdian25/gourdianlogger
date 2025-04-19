package main

import (
	"log"

	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Define a logger config with JSON format
	config := gourdianlogger.LoggerConfig{
		Filename:        "fmp_backend",
		MaxBytes:        10 * 1024 * 1024, // 10 MB
		BackupCount:     5,
		LogLevelStr:     "info",
		TimestampFormat: "2006-01-02 15:04:05",
		LogsDir:         "logs",
		EnableCaller:    true,
		AsyncWorkers:    4,
		FormatStr:       "json",
		EnableFallback:  true,
		MaxLogRate:      100,
		CompressBackups: true,
		RotationTime:    24 * 60 * 60, // 1 day
		SampleRate:      1,
		CallerDepth:     3,
		FormatConfig: gourdianlogger.FormatConfig{
			PrettyPrint: true,
			CustomFields: map[string]interface{}{
				"service": "fmp_backend",
				"env":     "development",
			},
		},
	}

	// Initialize logger
	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	logger.Info("Service initializing")
	logger.Debugf("Config loaded: %+v", "example config")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")
	// logger.Fatal("This is a fatal message")
	logger.Debug("This is a debug message")
	logger.Info("Service started")
	logger.Debug("Debugging information")
	logger.Warn("Warning: Low disk space")
	logger.Error("Error: Unable to connect to database")
	// logger.Fatal("Fatal: Unrecoverable error occurred")

	// Structured log with additional context
	logger.InfofWithFields(map[string]interface{}{
		"user_id":   123,
		"operation": "signup",
	}, "User registration completed")
	logger.DebugfWithFields(map[string]interface{}{
		"request_id": "abc-123",
		"status":     "success",
	}, "Request processed successfully")
	logger.ErrorfWithFields(map[string]interface{}{
		"request_id": "abc-123",
		"error":      "database connection failed",
	}, "Error processing request")
	logger.WarnfWithFields(map[string]interface{}{
		"request_id": "abc-123",
		"status":     "warning",
	}, "Request took longer than expected")
	logger.InfofWithFields(map[string]interface{}{
		"request_id": "abc-123",
		"status":     "info",
	}, "Request completed successfully")
}
