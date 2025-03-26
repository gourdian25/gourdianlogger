package gourdianlogger

// import (
// 	"io"
// 	"io/ioutil"
// 	"os"
// 	"strings"
// 	"testing"
// )

// func BenchmarkLogging(b *testing.B) {
// 	config := DefaultConfig()
// 	config.Outputs = []io.Writer{ioutil.Discard}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		b.Fatal(err)
// 	}
// 	defer logger.Close()

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		logger.Info("benchmark log message")
// 	}
// }

// func BenchmarkLoggingWithCaller(b *testing.B) {
// 	config := DefaultConfig()
// 	config.Outputs = []io.Writer{ioutil.Discard}
// 	config.EnableCaller = true
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		b.Fatal(err)
// 	}
// 	defer logger.Close()

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		logger.Info("benchmark log message with caller")
// 	}
// }

// func BenchmarkAsyncLogging(b *testing.B) {
// 	config := DefaultConfig()
// 	config.Outputs = []io.Writer{ioutil.Discard}
// 	config.BufferSize = 1000
// 	config.AsyncWorkers = 4
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		b.Fatal(err)
// 	}
// 	defer logger.Close()

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		logger.Info("async benchmark log message")
// 	}
// 	b.StopTimer()
// 	logger.Flush()
// }

// func BenchmarkLogRotation(b *testing.B) {
// 	dir := "bench_rotate"
// 	os.RemoveAll(dir)
// 	defer os.RemoveAll(dir)

// 	config := DefaultConfig()
// 	config.LogsDir = dir
// 	config.MaxBytes = 100 // Small size to force frequent rotation

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		b.Fatal(err)
// 	}
// 	defer logger.Close()

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		logger.Info(strings.Repeat("a", 50)) // Ensure rotation happens
// 	}
// }
