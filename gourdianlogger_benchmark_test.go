package gourdianlogger

import (
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

// BenchmarkLogging benchmarks basic synchronous logging
func BenchmarkLogging(b *testing.B) {
	config := DefaultConfig()
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

// BenchmarkLoggingWithCaller benchmarks logging with caller info
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

// BenchmarkLoggingJSON benchmarks JSON formatted logging
func BenchmarkLoggingJSON(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	config.Format = FormatJSON
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark json log message")
	}
}

// BenchmarkLoggingJSONPretty benchmarks pretty-printed JSON logging
func BenchmarkLoggingJSONPretty(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	config.Format = FormatJSON
	config.FormatConfig.PrettyPrint = true
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark pretty json log message")
	}
}

// BenchmarkLoggingWithCustomFields benchmarks logging with custom fields
func BenchmarkLoggingWithCustomFields(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	config.FormatConfig.CustomFields = map[string]interface{}{
		"app":     "benchmark",
		"version": "1.0.0",
	}
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark log with custom fields")
	}
}

// BenchmarkAsyncLogging benchmarks async logging with buffer
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

// BenchmarkAsyncLoggingHighContention benchmarks async logging under high contention
func BenchmarkAsyncLoggingHighContention(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	config.BufferSize = 100
	config.AsyncWorkers = 2 // Fewer workers to increase contention
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("high contention async log message")
	}
	b.StopTimer()
	logger.Flush()
}

// BenchmarkConcurrentLogging benchmarks concurrent log writes
func BenchmarkConcurrentLogging(b *testing.B) {
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
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("concurrent benchmark log message")
		}
	})
	b.StopTimer()
	logger.Flush()
}

// BenchmarkLogRotation benchmarks log rotation performance
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

// BenchmarkMultiOutput benchmarks logging to multiple outputs
func BenchmarkMultiOutput(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard, io.Discard, io.Discard} // Multiple discard writers
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("multi-output benchmark log message")
	}
}

// BenchmarkFormattedLogging benchmarks formatted logging methods
func BenchmarkFormattedLogging(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	tests := []struct {
		name string
		fn   func(string, ...interface{})
	}{
		{"Debugf", logger.Debugf},
		{"Infof", logger.Infof},
		{"Warnf", logger.Warnf},
		{"Errorf", logger.Errorf},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tt.fn("formatted log %d", i)
			}
		})
	}
}

// BenchmarkLogLevelFiltering benchmarks the impact of log level filtering
func BenchmarkLogLevelFiltering(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	tests := []struct {
		name  string
		level LogLevel
	}{
		{"DebugLevel", DEBUG},
		{"InfoLevel", INFO},
		{"WarnLevel", WARN},
		{"ErrorLevel", ERROR},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			logger.SetLogLevel(tt.level)
			for i := 0; i < b.N; i++ {
				logger.Debug("debug message")
				logger.Info("info message")
				logger.Warn("warn message")
				logger.Error("error message")
			}
		})
	}
}

// BenchmarkHeavyLogContent benchmarks logging with large messages
func BenchmarkHeavyLogContent(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	largeMessage := strings.Repeat("This is a large log message. ", 100) // ~3KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(largeMessage)
	}
}

// BenchmarkLoggerCreation benchmarks logger initialization
func BenchmarkLoggerCreation(b *testing.B) {
	dir := "bench_create"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := DefaultConfig()
		config.LogsDir = dir
		logger, err := NewGourdianLogger(config)
		if err != nil {
			b.Fatal(err)
		}
		logger.Close()
	}
}

// BenchmarkWithConfig benchmarks JSON config parsing and logger creation
func BenchmarkWithConfig(b *testing.B) {
	dir := "bench_config"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	jsonConfig := `{
		"filename": "config_test",
		"logs_dir": "` + dir + `",
		"log_level": "WARN",
		"format": "JSON",
		"format_config": {
			"pretty_print": true,
			"custom_fields": {
				"app": "benchmark"
			}
		}
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger, err := WithConfig(jsonConfig)
		if err != nil {
			b.Fatal(err)
		}
		logger.Close()
	}
}

// BenchmarkLoggingWithManyOutputs benchmarks logging with many outputs
func BenchmarkLoggingWithManyOutputs(b *testing.B) {
	// Create 10 discard writers
	outputs := make([]io.Writer, 10)
	for i := range outputs {
		outputs[i] = io.Discard
	}

	config := DefaultConfig()
	config.Outputs = outputs
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("log message with many outputs")
	}
}

// BenchmarkLoggingUnderContention benchmarks logging under lock contention
func BenchmarkLoggingUnderContention(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	workers := 10
	iterations := b.N / workers
	if iterations == 0 {
		iterations = 1
	}

	b.ResetTimer()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				logger.Info("contention test message")
			}
		}()
	}
	wg.Wait()
}

func BenchmarkDifferentLogLevels(b *testing.B) {
	config := DefaultConfig()
	config.Outputs = []io.Writer{io.Discard}
	logger, err := NewGourdianLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	tests := []struct {
		name string
		fn   func(string)
	}{
		{"Debug", func(s string) { logger.Debug(s) }},
		{"Info", func(s string) { logger.Info(s) }},
		{"Warn", func(s string) { logger.Warn(s) }},
		{"Error", func(s string) { logger.Error(s) }},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tt.fn("log message")
			}
		})
	}
}
