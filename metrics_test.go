package logdot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics("test_api_key")

	if metrics == nil {
		t.Fatal("NewMetrics returned nil")
	}
}

func TestMetricsLastHTTPCodeDefault(t *testing.T) {
	metrics := NewMetrics("test_api_key")

	if metrics.LastHTTPCode() != -1 {
		t.Errorf("Expected default HTTP code -1, got %d", metrics.LastHTTPCode())
	}
}

func TestForEntity(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	if client == nil {
		t.Fatal("ForEntity returned nil")
	}

	if client.EntityID() != "entity-uuid-123" {
		t.Errorf("Expected entity ID 'entity-uuid-123', got '%s'", client.EntityID())
	}
}

func TestBoundMetricsLastHTTPCodeDefault(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	if client.LastHTTPCode() != -1 {
		t.Errorf("Expected default HTTP code -1, got %d", client.LastHTTPCode())
	}
}

func TestBoundMetricsSingleBatch(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	// Should start empty
	if client.BatchSize() != 0 {
		t.Errorf("Expected batch size 0, got %d", client.BatchSize())
	}

	// Begin single-metric batch
	client.BeginBatch("temperature", "celsius")
	client.Add(23.5, nil)
	client.Add(24.0, nil)
	client.Add(23.8, nil)

	if client.BatchSize() != 3 {
		t.Errorf("Expected batch size 3, got %d", client.BatchSize())
	}

	// End batch should clear
	client.EndBatch()
	if client.BatchSize() != 0 {
		t.Errorf("Expected batch size 0 after EndBatch, got %d", client.BatchSize())
	}
}

func TestBoundMetricsAddFailsWhenNotInBatch(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	err := client.Add(23.5, nil)
	if err == nil {
		t.Error("Expected error when adding without batch mode")
	}

	if !strings.Contains(client.LastError(), "batch mode") {
		t.Errorf("Expected error message to contain 'batch mode', got '%s'", client.LastError())
	}
}

func TestBoundMetricsMultiBatch(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	client.BeginMultiBatch()
	client.AddMetric("cpu", 45, "percent", nil)
	client.AddMetric("memory", 2048, "MB", nil)
	client.AddMetric("disk", 50, "GB", nil)

	if client.BatchSize() != 3 {
		t.Errorf("Expected batch size 3, got %d", client.BatchSize())
	}
}

func TestBoundMetricsAddMetricFailsWhenNotInMultiBatch(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	err := client.AddMetric("cpu", 45, "percent", nil)
	if err == nil {
		t.Error("Expected error when adding metric without multi-batch mode")
	}

	if !strings.Contains(client.LastError(), "multi-metric batch mode") {
		t.Errorf("Expected error to contain 'multi-metric batch mode', got '%s'", client.LastError())
	}
}

func TestBoundMetricsAddFailsInMultiBatchMode(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	client.BeginMultiBatch()
	err := client.Add(45, nil)
	if err == nil {
		t.Error("Expected error when using Add in multi-batch mode")
	}
}

func TestBoundMetricsClearBatch(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	client.BeginBatch("temperature", "celsius")
	client.Add(23.5, nil)
	client.ClearBatch()

	if client.BatchSize() != 0 {
		t.Errorf("Expected batch size 0 after ClearBatch, got %d", client.BatchSize())
	}
}

func TestBoundMetricsSendFailsInBatchMode(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	client.BeginBatch("temperature", "celsius")
	err := client.Send(context.Background(), "cpu", 50, "percent", nil)
	if err == nil {
		t.Error("Expected error when using Send in batch mode")
	}
}

func TestFormatTags(t *testing.T) {
	tags := map[string]interface{}{
		"env":     "prod",
		"version": "1.0.0",
	}

	formatted := formatTags(tags)

	if len(formatted) != 2 {
		t.Errorf("Expected 2 formatted tags, got %d", len(formatted))
	}

	// Check that tags are in "key:value" format
	hasEnv := false
	hasVersion := false
	for _, tag := range formatted {
		if tag == "env:prod" {
			hasEnv = true
		}
		if tag == "version:1.0.0" {
			hasVersion = true
		}
	}

	if !hasEnv {
		t.Error("Expected formatted tags to contain 'env:prod'")
	}
	if !hasVersion {
		t.Error("Expected formatted tags to contain 'version:1.0.0'")
	}
}

func TestFormatTagsNil(t *testing.T) {
	formatted := formatTags(nil)
	if formatted != nil {
		t.Errorf("Expected nil for nil tags, got %v", formatted)
	}
}

func TestFormatTagsEmpty(t *testing.T) {
	formatted := formatTags(map[string]interface{}{})
	if formatted != nil {
		t.Errorf("Expected nil for empty tags, got %v", formatted)
	}
}

func TestEntityManagementWithMockServer(t *testing.T) {
	// Create a mock server for entity operations
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "POST" && strings.Contains(r.URL.Path, "/entities") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":   "entity-uuid-123",
					"name": "test-service",
				},
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/entities/by-name/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":   "entity-uuid-123",
					"name": "test-service",
				},
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Note: This test demonstrates the expected behavior but won't actually
	// make calls to the mock server since we can't easily override the base URL.
	// In production, you would use dependency injection or interfaces.

	metrics := NewMetrics("test_api_key")

	// Test ForEntity returns a proper BoundMetrics
	client := metrics.ForEntity("entity-uuid-123")
	if client.EntityID() != "entity-uuid-123" {
		t.Errorf("Expected entity ID 'entity-uuid-123', got '%s'", client.EntityID())
	}
}

func TestBoundMetricsWithTags(t *testing.T) {
	metrics := NewMetrics("test_api_key")
	client := metrics.ForEntity("entity-uuid-123")

	client.BeginBatch("response_time", "ms")
	client.Add(123, map[string]interface{}{
		"endpoint": "/api/users",
		"method":   "GET",
	})
	client.Add(456, map[string]interface{}{
		"endpoint": "/api/users",
		"method":   "POST",
	})

	if client.BatchSize() != 2 {
		t.Errorf("Expected batch size 2, got %d", client.BatchSize())
	}
}
