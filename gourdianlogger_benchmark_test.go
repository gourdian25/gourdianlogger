// File: gourdianlogger_benchmark_test.go

package gourdianlogger

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

// BenchmarkDirectLogging benchmarks synchronous logging without buffering
func BenchmarkDirectLogging(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_direct",
		BufferSize:   0, // No buffering
		AsyncWorkers: 0,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark log message")
	}
	b.StopTimer()
}

// BenchmarkBufferedLogging benchmarks async logging with buffer
func BenchmarkBufferedLogging(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_buffered",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark log message")
	}
	b.StopTimer()
}

// BenchmarkConcurrentLogging benchmarks concurrent logging
func BenchmarkConcurrentLogging(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_concurrent",
		BufferSize:   10000,
		AsyncWorkers: 8,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	workers := 16
	perWorker := b.N / workers

	b.ResetTimer()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				logger.Info("Concurrent benchmark log message")
			}
		}()
	}
	wg.Wait()
	b.StopTimer()
}

// BenchmarkJSONFormat benchmarks JSON formatted logging
func BenchmarkJSONFormat(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_json",
		LogFormat:    FormatJSON,
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark JSON log message")
	}
	b.StopTimer()
}

// BenchmarkWithFields benchmarks logging with additional fields
func BenchmarkWithFields(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_fields",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	fields := map[string]interface{}{
		"user_id":    12345,
		"ip_address": "192.168.1.1",
		"request_id": "abc123",
		"timestamp":  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields(fields, "Benchmark log with fields")
	}
	b.StopTimer()
}

// BenchmarkRateLimited benchmarks logging with rate limiting
func BenchmarkRateLimited(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_ratelimited",
		BufferSize:   1000,
		AsyncWorkers: 4,
		MaxLogRate:   10000, // 10,000 logs per second
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark rate limited log message")
	}
	b.StopTimer()
}

// BenchmarkFileRotation benchmarks logging with file rotation
func BenchmarkFileRotation(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_rotation",
		MaxBytes:     1024, // Small size to force rotation
		BackupCount:  5,
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark log message that will trigger rotation")
	}
	b.StopTimer()
}

// BenchmarkCallerInfo benchmarks logging with caller info enabled
func BenchmarkCallerInfo(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_caller",
		EnableCaller: true,
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark log with caller info")
	}
	b.StopTimer()
}

// BenchmarkPlainTextFormat benchmarks plain text formatted logging
func BenchmarkPlainTextFormat(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_plain",
		LogFormat:    FormatPlain,
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark plain text log message")
	}
	b.StopTimer()
}

// BenchmarkMultiOutput benchmarks logging to multiple outputs
func BenchmarkMultiOutput(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_multioutput",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
		Outputs:      []io.Writer{buf1, buf2},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark multi-output log message")
	}
	b.StopTimer()
}

// BenchmarkLogLevelFilter benchmarks the overhead of log level filtering
func BenchmarkLogLevelFilter(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_loglevel",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     ERROR, // Only ERROR and above
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// These should all be filtered out
		logger.Debug("Debug message that should be filtered")
		logger.Info("Info message that should be filtered")
		logger.Warn("Warn message that should be filtered")
	}
	b.StopTimer()
}

// BenchmarkDynamicLogLevel benchmarks dynamic log level changes
func BenchmarkDynamicLogLevel(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logger_bench")
	if err != nil {
		b.Fatal(err)
	}

	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_dynamic",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	// Set up dynamic level function
	counter := 0
	levelFn := func() LogLevel {
		counter++
		if counter%2 == 0 {
			return DEBUG
		}
		return INFO
	}
	logger.SetDynamicLevelFunc(levelFn)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark dynamic log level message")
	}
	b.StopTimer()
}
