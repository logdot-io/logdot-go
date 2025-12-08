package logdot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	if logger.Hostname() != "test-service" {
		t.Errorf("Expected hostname 'test-service', got '%s'", logger.Hostname())
	}
}

func TestLoggerEmptyContext(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	ctx := logger.GetContext()
	if len(ctx) != 0 {
		t.Errorf("Expected empty context, got %v", ctx)
	}
}

func TestWithContext(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	contextLogger := logger.WithContext(map[string]interface{}{
		"user_id": 123,
	})

	ctx := contextLogger.GetContext()
	if ctx["user_id"] != 123 {
		t.Errorf("Expected user_id 123, got %v", ctx["user_id"])
	}

	// Original should be unchanged
	originalCtx := logger.GetContext()
	if len(originalCtx) != 0 {
		t.Errorf("Original context should be empty, got %v", originalCtx)
	}
}

func TestWithContextChaining(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	logger1 := logger.WithContext(map[string]interface{}{"user_id": 123})
	logger2 := logger1.WithContext(map[string]interface{}{"request_id": "abc"})

	ctx := logger2.GetContext()
	if ctx["user_id"] != 123 {
		t.Errorf("Expected user_id 123, got %v", ctx["user_id"])
	}
	if ctx["request_id"] != "abc" {
		t.Errorf("Expected request_id 'abc', got %v", ctx["request_id"])
	}
}

func TestWithContextOverwrite(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	logger1 := logger.WithContext(map[string]interface{}{"env": "dev"})
	logger2 := logger1.WithContext(map[string]interface{}{"env": "prod"})

	ctx := logger2.GetContext()
	if ctx["env"] != "prod" {
		t.Errorf("Expected env 'prod', got %v", ctx["env"])
	}
}

func TestGetContextReturnsCopy(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")
	loggerWithContext := logger.WithContext(map[string]interface{}{"key": "value"})

	ctx := loggerWithContext.GetContext()
	ctx["key"] = "modified"

	// Original should be unchanged
	originalCtx := loggerWithContext.GetContext()
	if originalCtx["key"] != "value" {
		t.Errorf("Expected 'value', got '%v'", originalCtx["key"])
	}
}

func TestBatchOperations(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	// Should start with empty batch
	if logger.BatchSize() != 0 {
		t.Errorf("Expected batch size 0, got %d", logger.BatchSize())
	}

	// Begin batch and add logs
	logger.BeginBatch()
	logger.Info(context.Background(), "message 1", nil)
	logger.Info(context.Background(), "message 2", nil)

	if logger.BatchSize() != 2 {
		t.Errorf("Expected batch size 2, got %d", logger.BatchSize())
	}

	// End batch should clear
	logger.EndBatch()
	if logger.BatchSize() != 0 {
		t.Errorf("Expected batch size 0 after EndBatch, got %d", logger.BatchSize())
	}
}

func TestClearBatchKeepsBatchMode(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	logger.BeginBatch()
	logger.Info(context.Background(), "message 1", nil)
	logger.ClearBatch()

	if logger.BatchSize() != 0 {
		t.Errorf("Expected batch size 0 after ClearBatch, got %d", logger.BatchSize())
	}

	// Should still be in batch mode
	logger.Info(context.Background(), "message 2", nil)
	if logger.BatchSize() != 1 {
		t.Errorf("Expected batch size 1 after adding in batch mode, got %d", logger.BatchSize())
	}
}

func TestLogMethods(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	defer server.Close()

	// We can't easily override the base URL in the current implementation,
	// so we'll just test that the methods don't panic and return proper types
	logger := NewLogger("test_api_key", "test-service")

	// Use batch mode to test that methods work without making HTTP calls
	logger.BeginBatch()

	ctx := context.Background()
	logger.Debug(ctx, "debug message", nil)
	logger.Info(ctx, "info message", nil)
	logger.Warn(ctx, "warn message", nil)
	logger.Error(ctx, "error message", nil)

	if logger.BatchSize() != 4 {
		t.Errorf("Expected 4 logs in batch, got %d", logger.BatchSize())
	}
}

func TestLogWithTags(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")

	logger.BeginBatch()
	logger.Info(context.Background(), "message", map[string]interface{}{
		"endpoint": "/api/users",
		"method":   "GET",
	})

	if logger.BatchSize() != 1 {
		t.Errorf("Expected batch size 1, got %d", logger.BatchSize())
	}
}

func TestMergeTagsWithContext(t *testing.T) {
	logger := NewLogger("test_api_key", "test-service")
	contextLogger := logger.WithContext(map[string]interface{}{
		"service": "api",
		"env":     "prod",
	})

	// Private method test - verify context is set correctly
	ctx := contextLogger.GetContext()
	if ctx["service"] != "api" {
		t.Errorf("Expected service 'api', got '%v'", ctx["service"])
	}
	if ctx["env"] != "prod" {
		t.Errorf("Expected env 'prod', got '%v'", ctx["env"])
	}
}
