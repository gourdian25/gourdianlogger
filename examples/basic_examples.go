package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gourdian25/gourdianlogger"
)

func main() {
	// Run examples sequentially
	examples := []func(){
		simpleLoggerExample,
		customLoggerExample,
		asyncLoggerExample,
		jsonConfigLoggerExample,
		simulateAppActivity,
		dynamicConfigurationDemo,
	}

	for i, example := range examples {
		fmt.Printf("\n=== Running Example %d ===\n", i+1)
		example()
		time.Sleep(500 * time.Millisecond) // Pause between examples
	}
}

func simpleLoggerExample() {
	logger, err := gourdianlogger.NewGourdianLogger(gourdianlogger.DefaultConfig())
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		return
	}
	defer logger.Close()

	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")
	logger.Infof("Formatted message at %s", time.Now().Format(time.RFC3339))
}

func customLoggerExample() {
	config := gourdianlogger.LoggerConfig{
		Filename:        "myapp",
		MaxBytes:        5 * 1024 * 1024,
		BackupCount:     3,
		LogLevel:        gourdianlogger.INFO,
		TimestampFormat: "2006-01-02 15:04:05",
		LogsDir:         "custom_logs",
		EnableCaller:    true,
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Add stderr as additional output
	logger.AddOutput(os.Stderr)

	logger.Info("This will appear")
	logger.Warn("This will appear")
	logger.Debug("This won't appear")
}

func asyncLoggerExample() {
	config := gourdianlogger.LoggerConfig{
		Filename:     "async_app",
		MaxBytes:     2 * 1024 * 1024,
		BackupCount:  5,
		LogLevel:     gourdianlogger.DEBUG,
		BufferSize:   1000,
		AsyncWorkers: 2,
		ShowBanner:   true,
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		return
	}
	defer logger.Close()

	for i := 0; i < 100; i++ {
		logger.Infof("Processing item %d", i)
	}

	logger.Info("Final message before flush")
	logger.Flush()
}

func jsonConfigLoggerExample() {
	jsonConfig := `{
		"filename": "json_config_app",
		"max_bytes": 3145728,
		"backup_count": 7,
		"log_level": "WARN",
		"timestamp_format": "2006-01-02T15:04:05.000Z07:00",
		"logs_dir": "json_logs",
		"enable_caller": false,
		"buffer_size": 500,
		"async_workers": 3,
		"show_banner": false
	}`

	logger, err := gourdianlogger.WithConfig(jsonConfig)
	if err != nil {
		fmt.Printf("Error creating logger from JSON: %v\n", err)
		return
	}
	defer logger.Close()

	logger.Debug("This won't appear")
	logger.Warn("This will appear")
	logger.SetLogLevel(gourdianlogger.DEBUG)
	logger.Debug("Now this will appear")
}

type httpRequestLogger struct {
	outputs []io.Writer
}

func (h *httpRequestLogger) Write(p []byte) (n int, err error) {
	msg := fmt.Sprintf("[HTTP] %s", string(p))
	for _, w := range h.outputs {
		w.Write([]byte(msg))
	}
	return len(p), nil
}

func simulateAppActivity() {
	config := gourdianlogger.LoggerConfig{
		Filename:     "advanced_app",
		MaxBytes:     10 * 1024 * 1024,
		BackupCount:  10,
		LogLevel:     gourdianlogger.DEBUG,
		EnableCaller: true,
		BufferSize:   2000,
		AsyncWorkers: 4,
		ShowBanner:   true,
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Create HTTP logger that writes to stdout
	httpLogger := &httpRequestLogger{outputs: []io.Writer{os.Stdout}}
	logger.AddOutput(httpLogger)

	var wg sync.WaitGroup

	components := []struct {
		name  string
		count int
		delay time.Duration
	}{
		{"AUTH", 20, 100 * time.Millisecond},
		{"DB", 15, 150 * time.Millisecond},
		{"CACHE", 10, 200 * time.Millisecond},
	}

	for _, comp := range components {
		wg.Add(1)
		go func(name string, count int, delay time.Duration) {
			defer wg.Done()
			for i := 0; i < count; i++ {
				if name == "CACHE" && i%3 == 0 {
					logger.Warnf("[%s] Cache miss occurred %d", name, i)
				} else {
					logger.Infof("[%s] Operation %d", name, i)
				}
				time.Sleep(delay)
			}
		}(comp.name, comp.count, comp.delay)
	}

	wg.Wait()
}

func dynamicConfigurationDemo() {
	config := gourdianlogger.LoggerConfig{
		Filename:     "dynamic_app",
		MaxBytes:     5 * 1024 * 1024,
		BackupCount:  5,
		LogLevel:     gourdianlogger.INFO,
		EnableCaller: false,
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Create a buffer for temporary capture
	var buf bytes.Buffer
	logger.AddOutput(&buf)

	logger.Info("This goes to buffer")
	logger.Warn("This also goes to buffer")

	// Remove buffer before reading its contents
	logger.RemoveOutput(&buf)
	fmt.Println("\nCaptured logs:")
	fmt.Println(buf.String())

	// Demonstrate dynamic level changes
	logger.SetLogLevel(gourdianlogger.ERROR)
	logger.Debug("This won't appear")
	logger.Info("This won't appear")
	logger.Error("This will appear")

	logger.SetLogLevel(gourdianlogger.DEBUG)
	logger.Debug("Debug is back!")
}
