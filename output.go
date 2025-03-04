package gourdianlogger

import "io"

// AddOutput adds a new output destination for logging.
//
// Parameters:
//   - output: New output destination (e.g., external service writer)
//
// Example:
//
//	logger.AddOutput(externalServiceWriter)
func (l *Logger) AddOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputs = append(l.outputs, output)
	l.multiWriter = io.MultiWriter(l.outputs...)
}
