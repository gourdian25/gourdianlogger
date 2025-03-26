package gourdianlogger

// import (
// 	"io"
// 	"os"
// 	"testing"
// )

// func TestInvalidLogDir(t *testing.T) {
// 	config := DefaultConfig()
// 	config.LogsDir = "/invalid/path"

// 	_, err := NewGourdianLogger(config)
// 	if err == nil {
// 		t.Error("Expected error for invalid log directory")
// 	}
// }

// func TestFileWriteError(t *testing.T) {
// 	// Create a read-only file
// 	f, err := os.CreateTemp("", "readonly")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.Remove(f.Name())
// 	f.Close()
// 	os.Chmod(f.Name(), 0400)

// 	config := DefaultConfig()
// 	config.Outputs = []io.Writer{f}

// 	logger, err := NewGourdianLogger(config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer logger.Close()

// 	// This should fail but not panic
// 	logger.Info("test message")
// }

// func TestWithInvalidConfig(t *testing.T) {
// 	_, err := WithConfig("invalid json")
// 	if err == nil {
// 		t.Error("Expected error for invalid JSON config")
// 	}
// }

// func TestClosedLogger(t *testing.T) {
// 	logger, err := NewGourdianLogger(DefaultConfig())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	logger.Close()

// 	// These should not panic
// 	logger.Info("test")
// 	logger.AddOutput(os.Stdout)
// 	logger.RemoveOutput(os.Stdout)
// }
