package gourdianlogger

// import (
// 	"encoding/json"
// 	"testing"
// )

// func TestLogLevelSerialization(t *testing.T) {
// 	tests := []struct {
// 		level    string
// 		expected LogLevel
// 	}{
// 		{"DEBUG", DEBUG},
// 		{"INFO", INFO},
// 		{"WARN", WARN},
// 		{"ERROR", ERROR},
// 		{"FATAL", FATAL},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.level, func(t *testing.T) {
// 			got, err := ParseLogLevel(tt.level)
// 			if err != nil {
// 				t.Fatalf("ParseLogLevel failed: %v", err)
// 			}
// 			if got != tt.expected {
// 				t.Errorf("Expected %v, got %v", tt.expected, got)
// 			}

// 			if got.String() != tt.level {
// 				t.Errorf("String() mismatch: expected %s, got %s", tt.level, got.String())
// 			}
// 		})
// 	}
// }

// func TestConfigSerialization(t *testing.T) {
// 	config := DefaultConfig()
// 	configJson, err := json.Marshal(config)
// 	if err != nil {
// 		t.Fatalf("Marshal failed: %v", err)
// 	}

// 	var decodedConfig LoggerConfig
// 	if err := json.Unmarshal(configJson, &decodedConfig); err != nil {
// 		t.Fatalf("Unmarshal failed: %v", err)
// 	}

// 	if decodedConfig.Filename != config.Filename {
// 		t.Errorf("Filename mismatch: expected %s, got %s", config.Filename, decodedConfig.Filename)
// 	}
// }
