package gourdianlogger

import "testing"

// BenchmarkLogging benchmarks the performance of logging
func BenchmarkLogging(b *testing.B) {
	defer cleanupTestFiles(b)

	config := LoggerConfig{
		Filename:    "benchmark",
		LogsDir:     testLogDir,
		BackupCount: 1,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark log message")
	}
}

// BenchmarkAsyncLogging benchmarks the performance of async logging
func BenchmarkAsyncLogging(b *testing.B) {
	defer cleanupTestFiles(b)

	config := LoggerConfig{
		Filename:     "benchmark_async",
		LogsDir:      testLogDir,
		BackupCount:  1,
		BufferSize:   1000,
		AsyncWorkers: 4,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark async log message")
	}
	logger.Flush()
}

// BenchmarkLogRotation benchmarks the performance of log rotation
func BenchmarkLogRotation(b *testing.B) {
	defer cleanupTestFiles(b)

	config := LoggerConfig{
		Filename:    "benchmark_rotate",
		LogsDir:     testLogDir,
		MaxBytes:    100, // Small size to force rotation
		BackupCount: 5,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.mu.Lock()
		err := logger.rotateLogFiles()
		logger.mu.Unlock()
		if err != nil {
			b.Fatalf("Rotation failed: %v", err)
		}
	}
}
