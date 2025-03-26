package gourdianlogger

// import (
// 	"bytes"
// 	"sync"
// 	"testing"
// )

// func TestConcurrentRotation(t *testing.T) {
// 	config := DefaultConfig()
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	for i := 0; i < 10; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for j := 0; j < 100; j++ {
// 				logger.Info("test message")
// 			}
// 			logger.rotateChan <- struct{}{}
// 		}()
// 	}
// 	wg.Wait()
// }

// func TestConcurrentClose(t *testing.T) {
// 	config := DefaultConfig()
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var wg sync.WaitGroup
// 	wg.Add(2)
// 	go func() {
// 		defer wg.Done()
// 		logger.Close()
// 	}()
// 	go func() {
// 		defer wg.Done()
// 		logger.Info("message during close")
// 	}()
// 	wg.Wait()
// }

// func TestRaceLogLevel(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	go func() {
// 		defer wg.Done()
// 		logger.SetLogLevel(INFO)
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		_ = logger.GetLogLevel()
// 	}()

// 	wg.Wait()
// }

// func TestRaceConcurrentWrites(t *testing.T) {
// 	config := DefaultConfig()
// 	config.BufferSize = 1000
// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	for i := 0; i < 100; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			logger.Infof("message %d", i)
// 		}(i)
// 	}
// 	wg.Wait()
// }

// func TestRaceAddRemoveOutput(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	buf := new(bytes.Buffer)

// 	go func() {
// 		defer wg.Done()
// 		logger.AddOutput(buf)
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		logger.RemoveOutput(buf)
// 	}()

// 	wg.Wait()
// }
