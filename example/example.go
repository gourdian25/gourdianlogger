package main

import (
	"github.com/gourdian25/gourdianlogger"
)

func main() {
	logger, _ := gourdianlogger.NewGourdianLoggerWithDefault()
	defer logger.Close()

	// With fields
	logger.InfoWithFields(map[string]interface{}{
		"user_id": 123,
		"action":  "login",
	}, "User logged in")

	// Formatted with fields
	logger.InfofWithFields(map[string]interface{}{
		"duration_ms": 45,
		"method":      "GET",
		"path":        "/api/users",
	}, "Request processed in %dms", 45)
}
