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
	// examples := []func(){
	// 	simpleLoggerExample,
	// 	customLoggerExample,
	// 	asyncLoggerExample,
	// 	jsonConfigLoggerExample,
	// 	simulateAppActivity,
	// 	dynamicConfigurationDemo,
	// 	plainTextExample,
	// 	jsonExample,
	// 	logfmtExample,
	// 	csvExample,
	// 	xmlExample,
	// 	clfExample,
	// 	gelfExample,
	// 	cefExample,
	// 	jsonConfigExample,
	// }

	// for i, example := range examples {
	// 	fmt.Printf("\n=== Running Example %d ===\n", i+1)
	// 	example()
	// 	time.Sleep(500 * time.Millisecond)
	// }

	// Create the json_logs directory first to ensure it exists
	// os.MkdirAll("json_logs", 0755)

	jsonConfigExample()
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
		Filename:        "custom_logger",
		MaxBytes:        5 * 1024 * 1024,
		BackupCount:     3,
		LogLevel:        gourdianlogger.INFO,
		TimestampFormat: "2006-01-02 15:04:05",
		LogsDir:         "logs",
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
		Filename:     "async_logger",
		MaxBytes:     2 * 1024 * 1024,
		BackupCount:  5,
		LogLevel:     gourdianlogger.DEBUG,
		BufferSize:   1000,
		AsyncWorkers: 2,
		Format:       gourdianlogger.FormatJSON,
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
	// JSON configuration example
	jsonConfig := `{
        "filename": "json_config_logger",
        "max_bytes": 3145728,
        "backup_count": 7,
        "log_level": "WARN",
        "timestamp_format": "2006-01-02T15:04:05.000Z07:00",
        "logs_dir": "logs",
        "enable_caller": false,
        "buffer_size": 500,
        "async_workers": 3,
        "show_banner": false
		"format": "JSON",
    }`

	// Create logger from JSON config
	logger, err := gourdianlogger.WithConfig(jsonConfig)
	if err != nil {
		fmt.Printf("Failed to create logger from JSON: %v\n", err)
		return
	}
	defer logger.Close()

	// Test logging - these should appear in both console and file
	logger.Warn("This warning will appear in file and console")
	logger.Error("This error will appear in file and console")

	// Change log level dynamically
	logger.SetLogLevel(gourdianlogger.DEBUG)
	logger.Debug("Now debug messages will appear after level change")

	// Flush to ensure all messages are written
	logger.Flush()

	fmt.Println("Check the json_logs directory for output files")
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
		Filename:     "simulate_app_activity",
		MaxBytes:     10 * 1024 * 1024,
		BackupCount:  10,
		LogLevel:     gourdianlogger.DEBUG,
		EnableCaller: true,
		BufferSize:   2000,
		AsyncWorkers: 4,
		Format:       gourdianlogger.FormatPlain,
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
		Filename:     "dynamic_configuration",
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

func plainTextExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "plain_log"
	config.Format = gourdianlogger.FormatPlain

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("This is a plain text log entry")
	logger.Warn("Warning with caller info")
}

func jsonExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "json_log"
	config.Format = gourdianlogger.FormatJSON
	config.FormatConfig = gourdianlogger.FormatConfig{
		PrettyPrint: true,
		CustomFields: map[string]interface{}{
			"app": "myapp",
			"env": "production",
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("This is a JSON log entry")
	logger.Error("Error with stack trace")
}

func logfmtExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "logfmt_log"
	config.Format = gourdianlogger.FormatLogfmt
	config.FormatConfig = gourdianlogger.FormatConfig{
		CustomFields: map[string]interface{}{
			"service": "auth",
			"version": "2.1.0",
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("User logged in")
	logger.Warn("Failed login attempt")
}

func csvExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "csv_log"
	config.Format = gourdianlogger.FormatCSV
	config.FormatConfig = gourdianlogger.FormatConfig{
		CSVHeaders:   true,
		CSVDelimiter: '|', // Pipe delimiter instead of comma
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("Starting service")
	logger.Info("Processing request")
	logger.Error("Database connection failed")
}

func xmlExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "xml_log"
	config.Format = gourdianlogger.FormatXML
	config.FormatConfig = gourdianlogger.FormatConfig{
		PrettyPrint: true,
		CustomFields: map[string]interface{}{
			"deployment": "cluster-1",
			"region":     "us-west",
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("System initialized")
	logger.Warn("High memory usage detected")
}
func clfExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "clf_log"
	config.Format = gourdianlogger.FormatCLF
	config.FormatConfig = gourdianlogger.FormatConfig{
		CustomFields: map[string]interface{}{
			"host": "api.example.com",
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("GET /api/users HTTP/1.1")
	logger.Info("POST /api/auth HTTP/1.1")
}
func gelfExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "gelf_log"
	config.Format = gourdianlogger.FormatGELF
	config.FormatConfig = gourdianlogger.FormatConfig{
		CustomFields: map[string]interface{}{
			"_service": "payment",
			"_pod":     "payment-7865d4f96-abc12",
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Info("Payment processed")
	logger.Error("Payment failed with invalid card")
}

func cefExample() {
	config := gourdianlogger.DefaultConfig()
	config.Filename = "cef_log"
	config.Format = gourdianlogger.FormatCEF
	config.FormatConfig = gourdianlogger.FormatConfig{
		CustomFields: map[string]interface{}{
			"src": "192.168.1.100",
			"dst": "10.0.0.5",
			"act": "login",
		},
	}

	logger, err := gourdianlogger.NewGourdianLogger(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Warn("Failed login attempt")
	logger.Error("Brute force attack detected")
}

func jsonConfigExample() {
	jsonConfig := `{
        "filename": "config_log",
        "max_bytes": 3145728,
        "backup_count": 7,
        "log_level": "INFO",
        "timestamp_format": "2006-01-02T15:04:05.000Z07:00",
        "logs_dir": "json_logs",
        "enable_caller": true,
        "buffer_size": 500,
        "async_workers": 3,
        "show_banner": false,
        "format": "GELF",
        "format_config": {
            "custom_fields": {
                "app": "inventory",
                "version": "3.2.1",
                "environment": "staging"
            }
        }
    }`

	logger, err := gourdianlogger.WithConfig(jsonConfig)
	if err != nil {
		fmt.Printf("Failed to create logger from JSON: %v\n", err)
		return
	}
	defer logger.Close()

	logger.Info("Inventory service started")
	logger.Warn("Low stock warning detected")
	logger.Error("Failed to connect to database")

	// Add some context
	logger.Infof("Current stock levels: %d items", 42)
	logger.Warnf("Only %d units remaining for product %s", 3, "ABC123")

	logger.Flush()
	fmt.Println("Logging completed. Check json_logs/config_log.log")
}
