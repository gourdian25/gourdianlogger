package gourdianlogger

// import (
// 	"bytes"
// 	"fmt"
// 	"io"
// 	"sync"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/require"
// )

// func TestRaceRotationWithLogging(t *testing.T) {
// 	config := DefaultConfig()
// 	config.Filename = fmt.Sprintf("race_test_%d", time.Now().UnixNano())
// 	config.MaxBytes = 100 // Small size to trigger rotation quickly
// 	config.BackupCount = 5
// 	config.CompressBackups = false // Disable compression for test speed

// 	logger, err := NewGourdianLogger(config)
// 	require.NoError(t, err)
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	done := make(chan struct{})
// 	wg.Add(2)

// 	// Continuously log while rotation happens
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 1000; i++ {
// 			select {
// 			case <-done:
// 				return
// 			default:
// 				logger.Info("message during rotation")
// 				time.Sleep(1 * time.Millisecond) // Prevent overwhelming
// 			}
// 		}
// 	}()

// 	// Trigger rotation multiple times
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 10; i++ {
// 			select {
// 			case <-done:
// 				return
// 			default:
// 				logger.mu.Lock()
// 				logger.rotateChan <- struct{}{}
// 				logger.mu.Unlock()
// 				time.Sleep(10 * time.Millisecond)
// 			}
// 		}
// 	}()

// 	// Wait with timeout
// 	select {
// 	case <-waitWithTimeout(&wg, 5*time.Second):
// 	case <-time.After(5 * time.Second):
// 		close(done)
// 		t.Fatal("Test timed out")
// 	}
// }

// func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) chan struct{} {
// 	ch := make(chan struct{})
// 	go func() {
// 		wg.Wait()
// 		close(ch)
// 	}()
// 	return ch
// }

// func TestRaceAsyncWorkerShutdown(t *testing.T) {
// 	config := DefaultConfig()
// 	config.BufferSize = 100
// 	config.AsyncWorkers = 4
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	// Flood the async queue
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 1000; i++ {
// 			logger.Info("message during shutdown")
// 		}
// 	}()

// 	// Close while messages are still being processed
// 	go func() {
// 		defer wg.Done()
// 		time.Sleep(10 * time.Millisecond)
// 		logger.Close()
// 	}()

// 	wg.Wait()
// }

// func TestRaceBufferPool(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	for i := 0; i < 100; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			logger.Info("concurrent pool usage")
// 			logger.Warn("another concurrent message")
// 		}()
// 	}
// 	wg.Wait()
// }

// func TestRaceMultiWriter(t *testing.T) {
// 	config := DefaultConfig()
// 	config.Outputs = []io.Writer{new(bytes.Buffer), new(bytes.Buffer)}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(3)

// 	// Add new output while logging
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.AddOutput(new(bytes.Buffer))
// 		}
// 	}()

// 	// Remove output while logging
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			for _, w := range logger.outputs {
// 				logger.RemoveOutput(w)
// 			}
// 		}
// 	}()

// 	// Log continuously
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 1000; i++ {
// 			logger.Info("message during writer changes")
// 		}
// 	}()

// 	wg.Wait()
// }

// func TestRaceLevelChanges(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(4)

// 	levels := []LogLevel{DEBUG, INFO, WARN, ERROR, FATAL}

// 	// Rapidly change log level
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.SetLogLevel(levels[i%len(levels)])
// 		}
// 	}()

// 	// Log at different levels
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.Debug("debug message")
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.Info("info message")
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.Error("error message")
// 		}
// 	}()

// 	wg.Wait()
// }

// func TestRaceCallerInfo(t *testing.T) {
// 	config := DefaultConfig()
// 	config.EnableCaller = true
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	for i := 0; i < 100; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			logger.Info("message with caller info")
// 			logger.Error("error with caller info")
// 		}()
// 	}
// 	wg.Wait()
// }

// func TestRaceFormatChanges(t *testing.T) {
// 	config := DefaultConfig()
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(3)

// 	// Change format while logging
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			if i%2 == 0 {
// 				logger.format = FormatPlain
// 			} else {
// 				logger.format = FormatJSON
// 			}
// 		}
// 	}()

// 	// Log plain messages
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 1000; i++ {
// 			logger.Info("plain message")
// 		}
// 	}()

// 	// Log JSON messages
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 1000; i++ {
// 			logger.Info("json message")
// 		}
// 	}()

// 	wg.Wait()
// }

// func TestRaceConfigChanges(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(4)

// 	// Toggle caller info
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.enableCaller = !logger.enableCaller
// 		}
// 	}()

// 	// Change timestamp format
// 	go func() {
// 		defer wg.Done()
// 		formats := []string{time.RFC3339, time.RFC822, time.RFC1123}
// 		for i := 0; i < 100; i++ {
// 			logger.timestampFormat = formats[i%len(formats)]
// 		}
// 	}()

// 	// Log messages
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 1000; i++ {
// 			logger.Info("message during config changes")
// 		}
// 	}()

// 	// Change format config
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.formatConfig.PrettyPrint = !logger.formatConfig.PrettyPrint
// 		}
// 	}()

// 	wg.Wait()
// }

// func TestRaceCloseWithPendingMessages(t *testing.T) {
// 	config := DefaultConfig()
// 	config.BufferSize = 1000
// 	config.AsyncWorkers = 4
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	// Flood the queue
// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 10000; i++ {
// 			logger.Info("message before close")
// 		}
// 	}()

// 	// Close while messages are still being processed
// 	go func() {
// 		defer wg.Done()
// 		time.Sleep(50 * time.Millisecond)
// 		logger.Close()
// 	}()

// 	wg.Wait()
// }
