// Package logdot provides a client for the LogDot cloud logging and metrics service.
package logdot

import "time"

// LogLevel represents log severity levels
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	APIKey         string
	Hostname       string
	Timeout        time.Duration
	RetryAttempts  int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	Debug          bool
}

// MetricsConfig holds configuration for the metrics client
type MetricsConfig struct {
	APIKey         string
	Timeout        time.Duration
	RetryAttempts  int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	Debug          bool
}

// Config is deprecated - use LoggerConfig or MetricsConfig instead
type Config struct {
	APIKey            string
	Hostname          string
	EntityName        string
	EntityDescription string
	Timeout           time.Duration
	RetryAttempts     int
	RetryBaseDelay    time.Duration
	RetryMaxDelay     time.Duration
	Debug             bool
}

// Entity represents an entity returned from create/get operations
type Entity struct {
	ID          string
	Name        string
	Description string
}

// CreateEntityOptions holds options for creating an entity
type CreateEntityOptions struct {
	Name        string
	Description string
	Metadata    map[string]interface{}
}

// LogEntry represents a single log entry
type LogEntry struct {
	Message  string                 `json:"message"`
	Level    LogLevel               `json:"severity"`
	Hostname string                 `json:"hostname,omitempty"`
	Tags     map[string]interface{} `json:"tags,omitempty"`
}

// MetricEntry represents a single metric entry
type MetricEntry struct {
	EntityID string   `json:"entity_id,omitempty"`
	Name     string   `json:"name"`
	Value    float64  `json:"value"`
	Unit     string   `json:"unit"`
	Tags     []string `json:"tags,omitempty"`
}

// BatchLogsPayload for batch log transmission
type BatchLogsPayload struct {
	Hostname string     `json:"hostname"`
	Logs     []LogEntry `json:"logs"`
}

// BatchMetricsPayload for batch metric transmission
type BatchMetricsPayload struct {
	EntityID string             `json:"entity_id"`
	Name     string             `json:"name,omitempty"`
	Metrics  []BatchMetricEntry `json:"metrics"`
}

// BatchMetricEntry for batch metric entry
type BatchMetricEntry struct {
	Name  string   `json:"name,omitempty"`
	Value float64  `json:"value"`
	Unit  string   `json:"unit"`
	Tags  []string `json:"tags,omitempty"`
}

// EntityPayload for creating entities
type EntityPayload struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Data struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"data"`
	Status string `json:"status"`
}
