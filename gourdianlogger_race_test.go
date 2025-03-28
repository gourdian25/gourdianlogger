package gourdianlogger

import (
	"bytes"
	"sync"
	"testing"
)

func TestConcurrentRotation(t *testing.T) {
	config := DefaultConfig()
	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				logger.Info("test message")
			}
			logger.rotateChan <- struct{}{}
		}()
	}
	wg.Wait()
}

func TestConcurrentClose(t *testing.T) {
	config := DefaultConfig()
	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		logger.Close()
	}()
	go func() {
		defer wg.Done()
		logger.Info("message during close")
	}()
	wg.Wait()
}

func TestRaceLogLevel(t *testing.T) {
	logger, err := NewGourdianLogger(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logger.SetLogLevel(INFO)
	}()

	go func() {
		defer wg.Done()
		_ = logger.GetLogLevel()
	}()

	wg.Wait()
}

func TestRaceConcurrentWrites(t *testing.T) {
	config := DefaultConfig()
	config.BufferSize = 1000
	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.Infof("message %d", i)
		}(i)
	}
	wg.Wait()
}

func TestRaceAddRemoveOutput(t *testing.T) {
	logger, err := NewGourdianLogger(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	buf := new(bytes.Buffer)

	go func() {
		defer wg.Done()
		logger.AddOutput(buf)
	}()

	go func() {
		defer wg.Done()
		logger.RemoveOutput(buf)
	}()

	wg.Wait()
}

// TestRaceConditions runs tests with race detector
func TestRaceConditions(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.LogsDir = "test_logs"
	config.Filename = "race_test"
	config.BufferSize = 100
	config.AsyncWorkers = 2

	logger, err := NewGourdianLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	var wg sync.WaitGroup

	// Test concurrent log writes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			logger.Infof("goroutine 1: %d", i)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			logger.Errorf("goroutine 2: %d", i)
		}
	}()

	// Test concurrent configuration changes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			logger.SetLogLevel(LogLevel(i % 5))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			logger.AddOutput(&buf)
			logger.RemoveOutput(&buf)
		}
	}()

	wg.Wait()
}
