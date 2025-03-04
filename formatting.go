package gourdianlogger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// formatLogMessage formats a log message with timestamp, level, and source information.
//
// This method:
// - Captures source file, line number, and function name
// - Adds microsecond-precision timestamp
// - Formats the message in a consistent layout
//
// Parameters:
//   - level: Severity level of the log message
//   - message: The actual log message
//
// Returns:
//   - string: Formatted log message with metadata
//
// Format:
//
//	timestamp [LEVEL] file:line (function): message
func (l *Logger) formatLogMessage(level LogLevel, message string) string {
	// Call at depth 4 to include more stack frames for proper caller identification
	pc, file, line, ok := runtime.Caller(4)
	if !ok {
		file = "unknown"
		line = 0
	}

	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		// Get the full function name
		fullFunc := fn.Name()
		// Extract just the function name without the package path
		if lastSlash := strings.LastIndexByte(fullFunc, '/'); lastSlash >= 0 {
			fullFunc = fullFunc[lastSlash+1:]
		}
		if lastDot := strings.LastIndexByte(fullFunc, '.'); lastDot >= 0 {
			funcName = fullFunc[lastDot+1:]
		} else {
			funcName = fullFunc
		}
	}

	file = filepath.Base(file)

	now := time.Now()
	return fmt.Sprintf("%s [%s] %s:%d (%s): %s\n",
		now.Format(l.timestampFormat),
		level,
		file,
		line,
		funcName,
		message,
	)
}
