package main

import "github.com/gourdian25/gourdianlogger"

func main() {
	logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.LoggerConfig{
		Filename:    "example.log",
		MaxBytes:    10 * 1024 * 1024, // 10MB
		BackupCount: 5,
		LogLevel:    gourdianlogger.DEBUG,
	})
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Debug("This is a debug message")
	logger.Info("This is an info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")
	// logger.Fatal("This is a fatal message") // Will exit the program
}
