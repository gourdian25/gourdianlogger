package gourdianlogger

import (
	"io"
	"os"
	"strings"
	"testing"
)

func BenchmarkLoggingWithCaller(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	config.EnableCaller = true
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark log message with caller")
	}
}

func BenchmarkAsyncLogging(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	config.BufferSize = 1000
	config.AsyncWorkers = 4
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("async benchmark log message")
	}
	b.StopTimer()
	logger.Flush()
}

func BenchmarkLogRotation(b *testing.B) {
	dir := "bench_rotate"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	config := DefaultConfig()
	config.LogsDir = dir
	config.MaxBytes = 100 // Small size to force frequent rotation

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(strings.Repeat("a", 50)) // Ensure rotation happens
	}
}

// BenchmarkLogging benchmarks the performance of logging
func BenchmarkLogging(b *testing.B) {
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "benchmark"
	config.Outputs = []io.Writer{io.Discard}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark log message")
	}
}

// BenchmarkConcurrentLogging benchmarks concurrent log writes
func BenchmarkConcurrentLogging(b *testing.B) {
	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "benchmark_concurrent"
	config.BufferSize = 1000
	config.AsyncWorkers = 4
	config.Outputs = []io.Writer{io.Discard}

	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("concurrent benchmark log message")
		}
	})
}
