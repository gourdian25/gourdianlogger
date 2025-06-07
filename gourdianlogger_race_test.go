// File: gourdianlogger_race_test.go

package gourdianlogger

// import (
// 	"bytes"
// 	"io"
// 	"os"
// 	"sync"
// 	"testing"
// 	"time"
// )

// // faultyWriter is an io.Writer that fails periodically for testing
// type faultyWriter struct {
// 	failEvery int
// 	counter   int
// 	mu        sync.Mutex
// }

// func (fw *faultyWriter) Write(p []byte) (n int, err error) {
// 	fw.mu.Lock()
// 	defer fw.mu.Unlock()
// 	fw.counter++
// 	if fw.counter%fw.failEvery == 0 {
// 		return 0, io.ErrShortWrite
// 	}
// 	return len(p), nil
// }

// type mockWriter struct{}

// func newMockWriter() (mockWriter, *os.File) {
// 	f, _ := os.CreateTemp("", "mockwriter")
// 	return mockWriter{}, f
// }

// func (m mockWriter) Write(p []byte) (n int, err error) {
// 	return len(p), nil
// }

// // TestConcurrentLogging tests concurrent logging from multiple goroutines
// func TestConcurrentLogging(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "concurrent_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	var wg sync.WaitGroup
// 	for i := 0; i < 100; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 				if i%10 == 0 {
// 					logger.SetLogLevel(LogLevel(i % 5))
// 				}
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// // TestConcurrentClose tests closing the logger while logs are still being written
// func TestConcurrentClose(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "close_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer os.RemoveAll("test_logs")

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 			}
// 		}(i)
// 	}

// 	// Close the logger while logs are still being written
// 	go func() {
// 		time.Sleep(time.Millisecond * 50)
// 		logger.Close()
// 	}()

// 	wg.Wait()
// }

// // TestConcurrentOutputModification tests adding/removing outputs while logging
// func TestConcurrentOutputModification(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "output_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	buf := &bytes.Buffer{}
// 	logger.AddOutput(buf)

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 				if j%20 == 0 {
// 					if i%2 == 0 {
// 						logger.AddOutput(&bytes.Buffer{})
// 					} else {
// 						logger.RemoveOutput(buf)
// 					}
// 				}
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// // TestConcurrentLevelChange tests changing log level while logging
// func TestConcurrentLevelChange(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "level_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 				if j%10 == 0 {
// 					logger.SetLogLevel(LogLevel(j % 5))
// 				}
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// // TestConcurrentDynamicLevel tests dynamic level function while logging
// func TestConcurrentDynamicLevel(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "dynamic_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	counter := 0
// 	logger.SetDynamicLevelFunc(func() LogLevel {
// 		counter++
// 		return LogLevel(counter % 5)
// 	})

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// // TestConcurrentFlush tests flushing while logs are being written
// func TestConcurrentFlush(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "flush_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 				if j%20 == 0 {
// 					go logger.Flush()
// 				}
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// // TestConcurrentConfigChange tests changing config while logging
// func TestConcurrentConfigChange(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "config_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 				if j%25 == 0 {
// 					newConfig := LoggerConfig{
// 						Filename:     "config_test",
// 						LogLevel:     LogLevel(j % 5),
// 						BufferSize:   500 + j,
// 						AsyncWorkers: 2 + (j % 3),
// 						LogsDir:      "test_logs",
// 					}
// 					// This would normally be unsafe, but we're testing for races
// 					logger.config = newConfig
// 				}
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// // TestConcurrentErrorHandling tests error handling under concurrent load
// func TestConcurrentErrorHandling(t *testing.T) {
// 	// Create a faulty writer that will fail periodically
// 	faultyWriter := &faultyWriter{failEvery: 10}
// 	config := LoggerConfig{
// 		Filename:       "error_test",
// 		LogLevel:       DEBUG,
// 		BufferSize:     1000,
// 		AsyncWorkers:   4,
// 		LogsDir:        "test_logs",
// 		Outputs:        []io.Writer{faultyWriter},
// 		EnableFallback: true,
// 		SampleRate:     1,
// 		CallerDepth:    3,
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("Message %d from goroutine %d", j, i)
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// // TestConcurrentJSONLogging tests concurrent JSON formatted logging
// func TestConcurrentJSONLogging(t *testing.T) {
// 	config := LoggerConfig{
// 		Filename:     "json_test",
// 		LogLevel:     DEBUG,
// 		BufferSize:   1000,
// 		AsyncWorkers: 4,
// 		LogsDir:      "test_logs",
// 		Format:       FormatJSON,
// 		SampleRate:   1,
// 		CallerDepth:  3,
// 		FormatConfig: FormatConfig{
// 			PrettyPrint: false,
// 			CustomFields: map[string]interface{}{
// 				"app": "test",
// 			},
// 		},
// 	}
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create logger: %v", err)
// 	}
// 	defer logger.Close()
// 	defer os.RemoveAll("test_logs")

// 	var wg sync.WaitGroup
// 	for i := 0; i < 50; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				fields := map[string]interface{}{
// 					"goroutine": i,
// 					"iteration": j,
// 				}
// 				logger.InfofWithFields(fields, "Message %d from goroutine %d", j, i)
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// func TestConcurrentLogging_NoRace(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatalf("failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	for i := 0; i < 20; i++ {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Infof("goroutine %d message %d", id, j)
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// }

// func TestDynamicLogLevel_SetGetRace(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatalf("failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	stop := make(chan struct{})

// 	wg.Add(2)

// 	go func() {
// 		defer wg.Done()
// 		for {
// 			select {
// 			case <-stop:
// 				return
// 			default:
// 				_ = logger.GetLogLevel()
// 			}
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.SetLogLevel(DEBUG)
// 			logger.SetLogLevel(INFO)
// 			logger.SetLogLevel(ERROR)
// 		}
// 		close(stop)
// 	}()

// 	wg.Wait()
// }

// func TestAddRemoveOutputRace(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatalf("failed to create logger: %v", err)
// 	}
// 	defer logger.Close()

// 	_, w := newMockWriter()
// 	defer w.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.AddOutput(w)
// 			time.Sleep(time.Millisecond)
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		for i := 0; i < 100; i++ {
// 			logger.RemoveOutput(w)
// 			time.Sleep(time.Millisecond)
// 		}
// 	}()

// 	wg.Wait()
// }
