package main

import (
	"log"

	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Create a logger with default configuration
	logger, err := gourdianlogger.NewGourdianLoggerWithDefault()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Basic logs at various levels
	logger.Debug("This is a debug message")       // visible in default config
	logger.Info("Server started successfully")    // general info
	logger.Warn("Disk space is running low")      // warning message
	logger.Error("Failed to connect to database") // error message

	// Structured log with additional context
	logger.InfofWithFields(map[string]interface{}{
		"user_id":   123,
		"operation": "signup",
	}, "User registration completed")
}
