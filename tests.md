# Comprehensive Test Plan for GourdianLogger Package

Based on the logger package you've shared, here's a detailed test plan to make it more robust:

## Unit Tests

### 1. Log Level Tests
- Test LogLevel String() conversion for all levels
- Test ParseLogLevel with valid and invalid inputs
- Test case sensitivity in ParseLogLevel
- Test setting/getting log level via atomic operations

### 2. Configuration Validation
- Test LoggerConfig.Validate() with:
  - Valid configurations
  - Negative values for all numeric fields
  - Invalid format strings
  - Empty/missing required fields

### 3. Logger Initialization
- Test NewGourdianLogger with:
  - Default configuration
  - Custom configuration
  - Invalid directory paths
  - Non-writable directories
  - Various output combinations
- Test NewDefaultGourdianLogger

### 4. Log Formatting Tests
- Test formatPlain with:
  - All log levels
  - With and without caller info
  - With and without fields
  - Special characters in messages
- Test formatJSON with:
  - Pretty print enabled/disabled
  - Custom fields
  - Nested field structures
  - Special characters
- Test getCallerInfo with:
  - EnableCaller true/false
  - Various call depths

### 5. Output Handling
- Test AddOutput with:
  - Valid writers
  - Nil writer
  - Multiple writers
- Test RemoveOutput with:
  - Existing writer
  - Non-existent writer
- Test multi-writer behavior

### 6. Async Logging
- Test asyncWorker with:
  - Single message
  - Batch of messages
  - Buffer full scenario
  - Close during operation
- Test writeBatch and writeBuffer with:
  - Normal writes
  - Error scenarios
  - Closed logger
- Test buffer pool behavior

### 7. Log Rotation
- Test rotateLogFiles with:
  - File size under/over limit
  - Permission issues
  - File rename failures
  - Multiple rotations
- Test cleanupOldBackups with:
  - More backups than allowed
  - Exactly backupCount backups
  - No backups
  - Permission issues

### 8. Error Handling
- Test error handling for:
  - File write errors
  - JSON marshaling errors
  - Rotation failures
  - Async queue full scenarios
- Test fallback writer behavior

### 9. Concurrency Tests
- Test concurrent logging from multiple goroutines
- Test log rotation during heavy logging
- Test level changes during logging
- Test output changes during logging

## Integration Tests

### 1. File System Integration
- Test complete lifecycle with actual file operations
- Verify file permissions and ownership
- Test log rotation with real files
- Verify backup cleanup behavior

### 2. Rate Limiting
- Test rate limiting with:
  - Different rate limits
  - Burst scenarios
  - Disabled limiter

### 3. Dynamic Level Function
- Test dynamic level changes during operation
- Test with frequently changing level function
- Test with nil level function

## Functional Tests

### 1. Logging Methods
- Test all public logging methods (Debug, Info, Warn, Error, Fatal)
- Test all *f variants
- Test all *WithFields variants

### 2. Lifecycle Management
- Test Close() behavior:
  - Normal close
  - Double close
  - Close with pending async logs
- Test Flush() behavior
- Test Pause/Resume functionality

### 3. Edge Cases
- Test with empty messages
- Test with very long messages
- Test with special characters
- Test with concurrent close and log operations

## Performance Tests

### 1. Benchmark Tests
- Benchmark synchronous logging
- Benchmark async logging with different queue sizes
- Benchmark formatting operations
- Benchmark with/without caller info
- Benchmark with/without fields

### 2. Stress Tests
- High volume logging
- Rapid log level changes
- Continuous rotation scenarios
- Memory usage under load

## Suggested Test Improvements

1. **Mock Filesystem**: Use an in-memory filesystem mock for more reliable file operation tests.

2. **Error Injection**: Create test writers that fail on purpose to test error handling paths.

3. **Concurrency Verification**: Use race detector and synchronization primitives to verify thread safety.

4. **Memory Leak Detection**: Verify buffer pool doesn't leak memory.

5. **Custom Error Handler Testing**: Test custom error handler behavior.

6. **Dynamic Configuration**: Test changing configuration at runtime.

7. **Log Format Verification**: Parse output logs to verify correct formatting.

8. **Timestamp Accuracy**: Verify timestamp precision and format.

This comprehensive test plan should help identify any weaknesses in the logger implementation and ensure it behaves correctly under all expected (and unexpected) conditions.