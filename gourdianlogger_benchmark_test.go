// File: gourdianlogger_benchmark_test.go

package gourdianlogger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
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

// BenchmarkPlainTextFormatSmallMessage benchmarks plain text logging with small messages
func BenchmarkPlainTextFormatSmallMessage(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:   tempDir,
		Filename:  "bench_small",
		LogFormat: FormatPlain,
		LogLevel:  DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	message := "This is a small log message"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(message)
	}
}

// BenchmarkJSONFormatSmallMessage benchmarks JSON logging with small messages
func BenchmarkJSONFormatSmallMessage(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:   tempDir,
		Filename:  "bench_json_small",
		LogFormat: FormatJSON,
		LogLevel:  DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	message := "This is a small JSON log message"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(message)
	}
}

// BenchmarkLoggingWithVaryingMessageSizes benchmarks with different message sizes
func BenchmarkLoggingWithVaryingMessageSizes(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_sizes",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	sizes := []struct {
		name    string
		message string
	}{
		{"Small", "small message"},
		{"Medium", strings.Repeat("medium message ", 20)},
		{"Large", strings.Repeat("large message ", 100)},
		{"VeryLarge", strings.Repeat("very large message ", 500)},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				logger.Info(size.message)
			}
		})
	}
}

// BenchmarkLoggingWithManyFields benchmarks logging with many fields
func BenchmarkLoggingWithManyFields(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_many_fields",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	fields := make(map[string]interface{})
	for i := 0; i < 50; i++ {
		fields[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields(fields, "message with many fields")
	}
}

// BenchmarkLoggingWithComplexFields benchmarks logging with complex field types
func BenchmarkLoggingWithComplexFields(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_complex_fields",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	type User struct {
		ID       int
		Name     string
		Email    string
		IsActive bool
	}

	fields := map[string]interface{}{
		"user": User{
			ID:       123,
			Name:     "John Doe",
			Email:    "john@example.com",
			IsActive: true,
		},
		"permissions": []string{"read", "write", "execute"},
		"metadata": map[string]interface{}{
			"created_at": time.Now(),
			"updated_at": time.Now(),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields(fields, "message with complex fields")
	}
}

// BenchmarkLoggerCreation benchmarks logger initialization time
func BenchmarkLoggerCreation(b *testing.B) {
	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := LoggerConfig{
			LogsDir:  tempDir,
			Filename: fmt.Sprintf("bench_create_%d", i),
			LogLevel: DEBUG,
		}
		logger, err := NewGourdianLogger(config)
		if err != nil {
			b.Fatal(err)
		}
		logger.Close()
	}
}

// BenchmarkLoggerWithCustomErrorHandler benchmarks logging with custom error handler
func BenchmarkLoggerWithCustomErrorHandler(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_error_handler",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
		ErrorHandler: func(err error) {
			// Do nothing, just measure the overhead
		},
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message with error handler")
	}
}

// BenchmarkDynamicLevelChange benchmarks frequent dynamic level changes
func BenchmarkDynamicLevelChange(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_dynamic_level",
		BufferSize:   1000,
		AsyncWorkers: 4,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	counter := 0
	logger.SetDynamicLevelFunc(func() LogLevel {
		counter++
		return LogLevel(counter % 5)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message with dynamic level")
	}
}

// BenchmarkLoggingWithRateLimiting benchmarks logging with rate limiting
func BenchmarkLoggingWithRateLimiting(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_rate_limit",
		BufferSize:   1000,
		AsyncWorkers: 4,
		MaxLogRate:   100000, // 100,000 logs per second
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("rate limited message")
	}
}

// BenchmarkLoggingWithPauseResume benchmarks pausing and resuming the logger
func BenchmarkLoggingWithPauseResume(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_pause_resume",
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
		if i%10 == 0 {
			logger.Pause()
		}
		if i%10 == 5 {
			logger.Resume()
		}
		logger.Info("message with pause/resume")
	}
}

// BenchmarkAddRemoveOutput benchmarks adding and removing outputs
func BenchmarkAddRemoveOutput(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:  tempDir,
		Filename: "bench_add_remove",
		LogLevel: DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	output := &bytes.Buffer{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.AddOutput(output)
		logger.RemoveOutput(output)
	}
}

// BenchmarkMixedOperations benchmarks mixed logging operations
func BenchmarkMixedOperations(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_mixed",
		BufferSize:   10000,
		AsyncWorkers: 8,
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.RunParallel(func(pb *testing.PB) {
		count := 0
		for pb.Next() {
			count++

			// Create a new map for each iteration to avoid concurrent access
			fields := map[string]interface{}{
				"goroutine": count % 10,
				"iteration": count,
			}

			switch count % 6 {
			case 0:
				logger.Debugf("debug message %d", count)
			case 1:
				logger.Infof("info message %d", count)
			case 2:
				logger.WarnWithFields(fields, "warning message")
			case 3:
				logger.Errorf("error message %d", count)
			case 4:
				logger.InfoWithFields(fields, "structured info")
			case 5:
				logger.SetLogLevel(LogLevel(count % 5))
			}
		}
	})
}

// BenchmarkHighContention benchmarks logging under high contention
func BenchmarkHighContention(b *testing.B) {
	tempDir := b.TempDir()
	config := LoggerConfig{
		LogsDir:      tempDir,
		Filename:     "bench_contention",
		BufferSize:   100,
		AsyncWorkers: 2, // Few workers to increase contention
		LogLevel:     DEBUG,
	}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	workers := 100
	iterations := b.N / workers
	if iterations == 0 {
		iterations = 1
	}

	b.ResetTimer()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				logger.Infof("message from worker %d, iteration %d", id, i)
			}
		}(w)
	}
	wg.Wait()
	b.StopTimer()
	logger.Flush()
}
