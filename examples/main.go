package main

import (
	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Create logger with default config (color and banner enabled)
	logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Test logging
	logger.Debug("This is a debug message")
	logger.Info("This is an info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")
	logger.Fatal("This is a fatal message") // Will exit program
}
