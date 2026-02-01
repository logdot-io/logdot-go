package logdot

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestMiddleware(overrides ...func(*MiddlewareConfig)) (http.Handler, *Logger) {
	logger := NewLogger("test_key", "test-service")
	logger.BeginBatch()

	cfg := DefaultMiddlewareConfig()
	cfg.Logger = logger

	for _, fn := range overrides {
		fn(&cfg)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/notfound" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/panic" {
			panic("test panic")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := Middleware(cfg)(inner)
	return handler, logger
}

func TestMiddlewareReturnsResponse(t *testing.T) {
	handler, _ := newTestMiddleware()

	req := httptest.NewRequest("GET", "/api/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got '%s'", rr.Body.String())
	}
}

func TestMiddlewareLogs2xxAsInfo(t *testing.T) {
	handler, logger := newTestMiddleware()

	req := httptest.NewRequest("GET", "/api/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	entry := logger.batchQueue[0]
	if entry.Level != LevelInfo {
		t.Errorf("expected level info, got %s", entry.Level)
	}
}

func TestMiddlewareLogs4xxAsWarn(t *testing.T) {
	handler, logger := newTestMiddleware()

	req := httptest.NewRequest("GET", "/notfound", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	entry := logger.batchQueue[0]
	if entry.Level != LevelWarn {
		t.Errorf("expected level warn, got %s", entry.Level)
	}
}

func TestMiddlewareLogs5xxAsError(t *testing.T) {
	handler, logger := newTestMiddleware()

	req := httptest.NewRequest("GET", "/error", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	entry := logger.batchQueue[0]
	if entry.Level != LevelError {
		t.Errorf("expected level error, got %s", entry.Level)
	}
}

func TestMiddlewareLogIncludesTags(t *testing.T) {
	handler, logger := newTestMiddleware()

	req := httptest.NewRequest("POST", "/api/orders", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	tags := logger.batchQueue[0].Tags
	if tags == nil {
		t.Fatal("expected tags to be non-nil")
	}
	if tags["http_method"] != "POST" {
		t.Errorf("expected http_method POST, got %v", tags["http_method"])
	}
	if tags["http_path"] != "/api/orders" {
		t.Errorf("expected http_path /api/orders, got %v", tags["http_path"])
	}
	if tags["http_status"] != 200 {
		t.Errorf("expected http_status 200, got %v", tags["http_status"])
	}
	if _, ok := tags["duration_ms"]; !ok {
		t.Error("expected duration_ms tag to be present")
	}
	if tags["source"] != "http_middleware" {
		t.Errorf("expected source http_middleware, got %v", tags["source"])
	}
}

func TestMiddlewareSkipsIgnoredPaths(t *testing.T) {
	handler, logger := newTestMiddleware(func(cfg *MiddlewareConfig) {
		cfg.IgnorePaths = []string{"/health", "/ready"}
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if logger.BatchSize() != 0 {
		t.Errorf("expected 0 log entries for ignored path, got %d", logger.BatchSize())
	}
}

func TestMiddlewareDefaultStatus200(t *testing.T) {
	logger := NewLogger("test_key", "test-service")
	logger.BeginBatch()

	cfg := DefaultMiddlewareConfig()
	cfg.Logger = logger

	// Handler that writes body without explicit WriteHeader
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("implicit 200"))
	})

	handler := Middleware(cfg)(inner)
	req := httptest.NewRequest("GET", "/api/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}
	if logger.batchQueue[0].Level != LevelInfo {
		t.Errorf("expected level info for implicit 200, got %s", logger.batchQueue[0].Level)
	}
}

func TestMiddlewareNeverPanics(t *testing.T) {
	handler, _ := newTestMiddleware()

	req := httptest.NewRequest("GET", "/panic", nil)
	rr := httptest.NewRecorder()

	// Should not panic — middleware recovers
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 after panic, got %d", rr.Code)
	}
}

func TestMiddlewareNoMetricsWhenNil(t *testing.T) {
	handler, logger := newTestMiddleware(func(cfg *MiddlewareConfig) {
		cfg.Metrics = nil
		cfg.LogMetrics = true
	})

	req := httptest.NewRequest("GET", "/api/users", nil)
	rr := httptest.NewRecorder()

	// Should not panic even though Metrics is nil
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	// Request should still be logged
	if logger.BatchSize() != 1 {
		t.Errorf("expected 1 log entry, got %d", logger.BatchSize())
	}
}

func TestMiddlewareMessageContainsRequestInfo(t *testing.T) {
	handler, logger := newTestMiddleware()

	req := httptest.NewRequest("DELETE", "/api/users/42", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	msg := logger.batchQueue[0].Message
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	// Message should contain method, path, and status
	if !containsAll(msg, "DELETE", "/api/users/42", "200") {
		t.Errorf("expected message to contain DELETE, /api/users/42, 200; got: %s", msg)
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
