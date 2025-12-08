package logdot

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Logger handles log transmission to LogDot
type Logger struct {
	http     *HTTPClient
	hostname string
	debug    bool
	logCtx   map[string]interface{}

	mu         sync.Mutex
	batchMode  bool
	batchQueue []LogEntry
}

// DefaultLoggerConfig returns a LoggerConfig with default values
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Timeout:        5 * time.Second,
		RetryAttempts:  3,
		RetryBaseDelay: 1 * time.Second,
		RetryMaxDelay:  30 * time.Second,
		Debug:          false,
	}
}

// NewLogger creates a new Logger instance
//
// Example:
//
//	logger := logdot.NewLogger("ilog_live_YOUR_API_KEY", "my-service")
//	logger.Info(ctx, "Application started", nil)
func NewLogger(apiKey, hostname string, opts ...LoggerOption) *Logger {
	config := DefaultLoggerConfig()
	config.APIKey = apiKey
	config.Hostname = hostname

	for _, opt := range opts {
		opt(&config)
	}

	return &Logger{
		http: NewHTTPClient(
			config.APIKey,
			config.Timeout,
			RetryConfig{
				MaxAttempts: config.RetryAttempts,
				BaseDelay:   config.RetryBaseDelay,
				MaxDelay:    config.RetryMaxDelay,
			},
			config.Debug,
		),
		hostname:   config.Hostname,
		debug:      config.Debug,
		logCtx:     make(map[string]interface{}),
		batchQueue: make([]LogEntry, 0),
	}
}

// LoggerOption is a function that configures a LoggerConfig
type LoggerOption func(*LoggerConfig)

// WithLoggerTimeout sets the HTTP timeout
func WithLoggerTimeout(timeout time.Duration) LoggerOption {
	return func(c *LoggerConfig) {
		c.Timeout = timeout
	}
}

// WithLoggerRetry sets retry configuration
func WithLoggerRetry(attempts int, baseDelay, maxDelay time.Duration) LoggerOption {
	return func(c *LoggerConfig) {
		c.RetryAttempts = attempts
		c.RetryBaseDelay = baseDelay
		c.RetryMaxDelay = maxDelay
	}
}

// WithLoggerDebug enables debug output
func WithLoggerDebug(enabled bool) LoggerOption {
	return func(c *LoggerConfig) {
		c.Debug = enabled
	}
}

// WithContext creates a new Logger with additional context that will be merged with all log tags.
// The returned logger shares the same HTTP client but has its own context.
//
// Example:
//
//	logger := logdot.NewLogger("apiKey", "my-service")
//	userLogger := logger.WithContext(map[string]interface{}{"user_id": 123})
//	userLogger.Info(ctx, "User action", nil) // Will include user_id
func (l *Logger) WithContext(ctx map[string]interface{}) *Logger {
	// Merge existing context with new context
	mergedCtx := make(map[string]interface{})
	for k, v := range l.logCtx {
		mergedCtx[k] = v
	}
	for k, v := range ctx {
		mergedCtx[k] = v
	}

	return &Logger{
		http:       l.http,
		hostname:   l.hostname,
		debug:      l.debug,
		logCtx:     mergedCtx,
		batchMode:  false,
		batchQueue: make([]LogEntry, 0),
	}
}

// GetContext returns a copy of the current context
func (l *Logger) GetContext() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make(map[string]interface{})
	for k, v := range l.logCtx {
		result[k] = v
	}
	return result
}

// mergeTags merges context with provided tags (tags take precedence)
func (l *Logger) mergeTags(tags map[string]interface{}) map[string]interface{} {
	if len(l.logCtx) == 0 && len(tags) == 0 {
		return nil
	}
	merged := make(map[string]interface{})
	for k, v := range l.logCtx {
		merged[k] = v
	}
	for k, v := range tags {
		merged[k] = v
	}
	return merged
}

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, message string, tags map[string]interface{}) error {
	return l.Log(ctx, LevelDebug, message, tags)
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, message string, tags map[string]interface{}) error {
	return l.Log(ctx, LevelInfo, message, tags)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, message string, tags map[string]interface{}) error {
	return l.Log(ctx, LevelWarn, message, tags)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, message string, tags map[string]interface{}) error {
	return l.Log(ctx, LevelError, message, tags)
}

// Log sends a log entry at the specified level
func (l *Logger) Log(ctx context.Context, level LogLevel, message string, tags map[string]interface{}) error {
	mergedTags := l.mergeTags(tags)
	entry := LogEntry{
		Message: message,
		Level:   level,
		Tags:    mergedTags,
	}

	l.mu.Lock()
	if l.batchMode {
		l.batchQueue = append(l.batchQueue, entry)
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()

	return l.sendLog(ctx, entry)
}

// BeginBatch starts batch mode
func (l *Logger) BeginBatch() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.batchMode = true
	l.batchQueue = make([]LogEntry, 0)
}

// SendBatch sends all queued logs
func (l *Logger) SendBatch(ctx context.Context) error {
	l.mu.Lock()
	if !l.batchMode || len(l.batchQueue) == 0 {
		l.mu.Unlock()
		return nil
	}

	logs := make([]LogEntry, len(l.batchQueue))
	copy(logs, l.batchQueue)
	l.mu.Unlock()

	payload := BatchLogsPayload{
		Hostname: l.hostname,
		Logs:     logs,
	}

	url := baseLogsURL + "/logs/batch"
	resp, _, err := l.http.Post(ctx, url, payload)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("batch send failed with status %d", resp.StatusCode)
	}

	l.ClearBatch()
	return nil
}

// EndBatch exits batch mode
func (l *Logger) EndBatch() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.batchMode = false
	l.batchQueue = make([]LogEntry, 0)
}

// ClearBatch clears the batch queue
func (l *Logger) ClearBatch() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.batchQueue = make([]LogEntry, 0)
}

// BatchSize returns the number of queued logs
func (l *Logger) BatchSize() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.batchQueue)
}

// Hostname returns the configured hostname
func (l *Logger) Hostname() string {
	return l.hostname
}

// SetDebug enables or disables debug output
func (l *Logger) SetDebug(enabled bool) {
	l.debug = enabled
}

func (l *Logger) sendLog(ctx context.Context, entry LogEntry) error {
	entry.Hostname = l.hostname

	url := baseLogsURL + "/logs"
	resp, _, err := l.http.Post(ctx, url, entry)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("log send failed with status %d", resp.StatusCode)
	}

	return nil
}

func (l *Logger) debugLog(message string) {
	if l.debug {
		fmt.Printf("[LogDotLogger] %s\n", message)
	}
}
