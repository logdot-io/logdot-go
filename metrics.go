package logdot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// BoundMetrics is a metrics client bound to a specific entity
type BoundMetrics struct {
	http     *HTTPClient
	entityID string
	debug    bool

	mu              sync.Mutex
	batchMode       bool
	multiBatchMode  bool
	batchMetricName string
	batchUnit       string
	batchQueue      []MetricEntry
	lastError       string
	lastHTTPCode    int
}

// Metrics handles entity management and metrics client creation
type Metrics struct {
	http  *HTTPClient
	debug bool

	lastError    string
	lastHTTPCode int
}

// DefaultMetricsConfig returns a MetricsConfig with default values
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Timeout:        5 * time.Second,
		RetryAttempts:  3,
		RetryBaseDelay: 1 * time.Second,
		RetryMaxDelay:  30 * time.Second,
		Debug:          false,
	}
}

// NewMetrics creates a new Metrics instance
//
// Example:
//
//	metrics := logdot.NewMetrics("ilog_live_YOUR_API_KEY")
//	entity, err := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{Name: "my-service"})
//	client := metrics.ForEntity(entity.ID)
//	client.Send(ctx, "cpu.usage", 45.5, "percent", nil)
func NewMetrics(apiKey string, opts ...MetricsOption) *Metrics {
	config := DefaultMetricsConfig()
	config.APIKey = apiKey

	for _, opt := range opts {
		opt(&config)
	}

	return &Metrics{
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
		debug:        config.Debug,
		lastHTTPCode: -1,
	}
}

// MetricsOption is a function that configures a MetricsConfig
type MetricsOption func(*MetricsConfig)

// WithMetricsTimeout sets the HTTP timeout
func WithMetricsTimeout(timeout time.Duration) MetricsOption {
	return func(c *MetricsConfig) {
		c.Timeout = timeout
	}
}

// WithMetricsRetry sets retry configuration
func WithMetricsRetry(attempts int, baseDelay, maxDelay time.Duration) MetricsOption {
	return func(c *MetricsConfig) {
		c.RetryAttempts = attempts
		c.RetryBaseDelay = baseDelay
		c.RetryMaxDelay = maxDelay
	}
}

// WithMetricsDebug enables debug output
func WithMetricsDebug(enabled bool) MetricsOption {
	return func(c *MetricsConfig) {
		c.Debug = enabled
	}
}

// CreateEntity creates a new entity
//
// Example:
//
//	entity, err := metrics.CreateEntity(ctx, logdot.CreateEntityOptions{
//		Name:        "my-service",
//		Description: "Production service",
//		Metadata:    map[string]interface{}{"version": "1.0.0"},
//	})
func (m *Metrics) CreateEntity(ctx context.Context, opts CreateEntityOptions) (*Entity, error) {
	payload := EntityPayload{
		Name:        opts.Name,
		Description: opts.Description,
		Metadata:    opts.Metadata,
	}

	reqURL := baseMetricsURL + "/entities"
	resp, body, err := m.http.Post(ctx, reqURL, payload)
	if err != nil {
		m.lastError = err.Error()
		return nil, err
	}

	m.lastHTTPCode = resp.StatusCode

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		m.lastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return nil, fmt.Errorf("entity creation failed with status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		m.lastError = err.Error()
		return nil, err
	}

	if apiResp.Data.ID == "" {
		m.lastError = "no entity ID in response"
		return nil, fmt.Errorf("no entity ID in response")
	}

	m.lastError = ""
	m.debugLog(fmt.Sprintf("Entity created: %s", apiResp.Data.ID))

	return &Entity{
		ID:          apiResp.Data.ID,
		Name:        opts.Name,
		Description: opts.Description,
	}, nil
}

// GetEntityByName retrieves an entity by name
//
// Example:
//
//	entity, err := metrics.GetEntityByName(ctx, "my-service")
//	if entity != nil {
//		client := metrics.ForEntity(entity.ID)
//	}
func (m *Metrics) GetEntityByName(ctx context.Context, name string) (*Entity, error) {
	encodedName := url.PathEscape(name)
	reqURL := fmt.Sprintf("%s/entities/by-name/%s", baseMetricsURL, encodedName)

	resp, body, err := m.http.Get(ctx, reqURL)
	if err != nil {
		m.lastError = err.Error()
		return nil, err
	}

	m.lastHTTPCode = resp.StatusCode

	if resp.StatusCode != 200 {
		m.lastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return nil, fmt.Errorf("entity not found (status %d)", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		m.lastError = err.Error()
		return nil, err
	}

	if apiResp.Data.ID == "" {
		m.lastError = "no entity ID in response"
		return nil, fmt.Errorf("no entity ID in response")
	}

	m.lastError = ""
	m.debugLog(fmt.Sprintf("Entity found: %s", apiResp.Data.ID))

	return &Entity{
		ID:          apiResp.Data.ID,
		Name:        apiResp.Data.Name,
		Description: apiResp.Data.Description,
	}, nil
}

// GetOrCreateEntity retrieves an existing entity or creates a new one
//
// Example:
//
//	entity, err := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{
//		Name: "my-service",
//	})
func (m *Metrics) GetOrCreateEntity(ctx context.Context, opts CreateEntityOptions) (*Entity, error) {
	// Try to find existing entity first
	entity, err := m.GetEntityByName(ctx, opts.Name)
	if err == nil && entity != nil {
		return entity, nil
	}

	// Create new entity
	return m.CreateEntity(ctx, opts)
}

// ForEntity creates a bound metrics client for a specific entity
//
// Example:
//
//	entity, _ := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{Name: "my-service"})
//	client := metrics.ForEntity(entity.ID)
//	client.Send(ctx, "cpu.usage", 45, "percent", nil)
func (m *Metrics) ForEntity(entityID string) *BoundMetrics {
	return &BoundMetrics{
		http:         m.http,
		entityID:     entityID,
		debug:        m.debug,
		batchQueue:   make([]MetricEntry, 0),
		lastHTTPCode: -1,
	}
}

// LastError returns the last error message
func (m *Metrics) LastError() string {
	return m.lastError
}

// LastHTTPCode returns the last HTTP response code
func (m *Metrics) LastHTTPCode() int {
	return m.lastHTTPCode
}

// SetDebug enables or disables debug output
func (m *Metrics) SetDebug(enabled bool) {
	m.debug = enabled
}

func (m *Metrics) debugLog(message string) {
	if m.debug {
		fmt.Printf("[LogDotMetrics] %s\n", message)
	}
}

// ============== BoundMetrics methods ==============

// EntityID returns the entity ID this client is bound to
func (b *BoundMetrics) EntityID() string {
	return b.entityID
}

// Send transmits a single metric
func (b *BoundMetrics) Send(ctx context.Context, name string, value float64, unit string, tags map[string]interface{}) error {
	b.mu.Lock()
	if b.batchMode {
		b.mu.Unlock()
		b.lastError = "cannot use Send() in batch mode"
		return fmt.Errorf("cannot use Send() in batch mode")
	}
	b.mu.Unlock()

	entry := MetricEntry{
		EntityID: b.entityID,
		Name:     name,
		Value:    value,
		Unit:     unit,
		Tags:     formatTags(tags),
	}

	reqURL := baseMetricsURL + "/metrics"
	resp, _, err := b.http.Post(ctx, reqURL, entry)
	if err != nil {
		b.lastError = err.Error()
		return err
	}

	b.mu.Lock()
	b.lastHTTPCode = resp.StatusCode
	b.mu.Unlock()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		b.lastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return fmt.Errorf("metric send failed with status %d", resp.StatusCode)
	}

	b.lastError = ""
	return nil
}

// BeginBatch starts single-metric batch mode
func (b *BoundMetrics) BeginBatch(metricName, unit string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batchMode = true
	b.multiBatchMode = false
	b.batchMetricName = metricName
	b.batchUnit = unit
	b.batchQueue = make([]MetricEntry, 0)
}

// Add adds a value to the current batch
func (b *BoundMetrics) Add(value float64, tags map[string]interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.batchMode || b.multiBatchMode {
		b.lastError = "not in single-metric batch mode"
		return fmt.Errorf("not in single-metric batch mode")
	}

	b.batchQueue = append(b.batchQueue, MetricEntry{
		Name:  b.batchMetricName,
		Value: value,
		Unit:  b.batchUnit,
		Tags:  formatTags(tags),
	})

	return nil
}

// BeginMultiBatch starts multi-metric batch mode
func (b *BoundMetrics) BeginMultiBatch() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batchMode = true
	b.multiBatchMode = true
	b.batchQueue = make([]MetricEntry, 0)
}

// AddMetric adds a metric to the multi-batch queue
func (b *BoundMetrics) AddMetric(name string, value float64, unit string, tags map[string]interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.multiBatchMode {
		b.lastError = "not in multi-metric batch mode"
		return fmt.Errorf("not in multi-metric batch mode")
	}

	b.batchQueue = append(b.batchQueue, MetricEntry{
		Name:  name,
		Value: value,
		Unit:  unit,
		Tags:  formatTags(tags),
	})

	return nil
}

// SendBatch sends all queued metrics
func (b *BoundMetrics) SendBatch(ctx context.Context) error {
	b.mu.Lock()
	if !b.batchMode || len(b.batchQueue) == 0 {
		b.mu.Unlock()
		return nil
	}

	metrics := make([]BatchMetricEntry, len(b.batchQueue))
	for i, entry := range b.batchQueue {
		metrics[i] = BatchMetricEntry{
			Value: entry.Value,
			Unit:  entry.Unit,
			Tags:  entry.Tags,
		}
		if b.multiBatchMode {
			metrics[i].Name = entry.Name
		}
	}

	payload := BatchMetricsPayload{
		EntityID: b.entityID,
		Metrics:  metrics,
	}

	if !b.multiBatchMode {
		payload.Name = b.batchMetricName
	}
	b.mu.Unlock()

	reqURL := baseMetricsURL + "/metrics/batch"
	resp, _, err := b.http.Post(ctx, reqURL, payload)
	if err != nil {
		b.lastError = err.Error()
		return err
	}

	b.mu.Lock()
	b.lastHTTPCode = resp.StatusCode
	b.mu.Unlock()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		b.lastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return fmt.Errorf("batch send failed with status %d", resp.StatusCode)
	}

	b.ClearBatch()
	b.lastError = ""
	return nil
}

// EndBatch exits batch mode
func (b *BoundMetrics) EndBatch() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batchMode = false
	b.multiBatchMode = false
	b.batchQueue = make([]MetricEntry, 0)
}

// ClearBatch clears the batch queue
func (b *BoundMetrics) ClearBatch() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batchQueue = make([]MetricEntry, 0)
}

// BatchSize returns the number of queued metrics
func (b *BoundMetrics) BatchSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.batchQueue)
}

// LastError returns the last error message
func (b *BoundMetrics) LastError() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastError
}

// LastHTTPCode returns the last HTTP response code
func (b *BoundMetrics) LastHTTPCode() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastHTTPCode
}

// SetDebug enables or disables debug output
func (b *BoundMetrics) SetDebug(enabled bool) {
	b.debug = enabled
}

// formatTags converts a map to a list of "key:value" strings
func formatTags(tags map[string]interface{}) []string {
	if tags == nil || len(tags) == 0 {
		return nil
	}
	result := make([]string, 0, len(tags))
	for key, value := range tags {
		result = append(result, fmt.Sprintf("%s:%v", key, value))
	}
	return result
}
