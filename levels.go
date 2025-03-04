package gourdianlogger

import (
	"errors"
	"strings"
)

// LogLevel represents the severity level of a log message.
// Higher values indicate more severe log levels.
type LogLevel int

// Log level constants defining the supported severity levels.
//
// Levels are ordered from least to most severe:
// - DEBUG: Detailed information for debugging
// - INFO: General operational information
// - WARN: Warning messages for potentially harmful situations
// - ERROR: Error messages for serious problems
// - FATAL: Critical errors causing program termination
const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String converts a LogLevel to its string representation.
//
// Returns:
//   - string: Uppercase string name of the log level
func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[l]
}

// ParseLogLevel converts a string to its corresponding LogLevel.
//
// Parameters:
//   - level: String representation of the log level (case-insensitive)
//
// Returns:
//   - LogLevel: Corresponding log level constant
//   - error: Error if the input string is not a valid log level
//
// Example:
//
//	level, err := ParseLogLevel("INFO")
//	if err != nil {
//	    panic(err)
//	}
//	fmt.Println(level) // Output: INFO
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	default:
		return DEBUG, errors.New("invalid log level: " + level)
	}
}
