package gourdianlogger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func (l *Logger) formatLogMessage(level LogLevel, message string) string {
	pc, file, line, ok := runtime.Caller(4)
	if !ok {
		file = "unknown"
		line = 0
	}

	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		fullFunc := fn.Name()
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
