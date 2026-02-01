package logdot

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func newTestSlogHandler(opts ...SlogHandlerOption) (*SlogHandler, *Logger) {
	logger := NewLogger("test_key", "test-service")
	logger.BeginBatch()
	h := NewSlogHandler(logger, opts...)
	return h, logger
}

func TestSlogHandlerForwardsInfo(t *testing.T) {
	h, logger := newTestSlogHandler()

	slogLogger := slog.New(h)
	slogLogger.Info("test info message")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	entry := logger.batchQueue[0]
	if entry.Level != LevelInfo {
		t.Errorf("expected level info, got %s", entry.Level)
	}
	if entry.Message != "test info message" {
		t.Errorf("expected message 'test info message', got '%s'", entry.Message)
	}
}

func TestSlogHandlerForwardsError(t *testing.T) {
	h, logger := newTestSlogHandler()
	slogLogger := slog.New(h)
	slogLogger.Error("test error")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}
	if logger.batchQueue[0].Level != LevelError {
		t.Errorf("expected level error, got %s", logger.batchQueue[0].Level)
	}
}

func TestSlogHandlerForwardsWarn(t *testing.T) {
	h, logger := newTestSlogHandler()
	slogLogger := slog.New(h)
	slogLogger.Warn("test warning")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}
	if logger.batchQueue[0].Level != LevelWarn {
		t.Errorf("expected level warn, got %s", logger.batchQueue[0].Level)
	}
}

func TestSlogHandlerForwardsDebug(t *testing.T) {
	h, logger := newTestSlogHandler()
	slogLogger := slog.New(h)
	// Debug is enabled by default in our handler
	slogLogger.Debug("test debug")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}
	if logger.batchQueue[0].Level != LevelDebug {
		t.Errorf("expected level debug, got %s", logger.batchQueue[0].Level)
	}
}

func TestSlogHandlerIncludesAttrs(t *testing.T) {
	h, logger := newTestSlogHandler()
	slogLogger := slog.New(h)
	slogLogger.Info("with attrs", "user_id", 42, "action", "login")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	tags := logger.batchQueue[0].Tags
	if tags == nil {
		t.Fatal("expected non-nil tags")
	}
	if tags["source"] != "slog" {
		t.Errorf("expected source slog, got %v", tags["source"])
	}

	// slog attrs should be in tags
	uid, ok := tags["user_id"]
	if !ok {
		t.Error("expected user_id tag")
	}
	// slog stores int as int64
	if v, ok := uid.(int64); !ok || v != 42 {
		t.Errorf("expected user_id 42 (int64), got %v (%T)", uid, uid)
	}
	if tags["action"] != "login" {
		t.Errorf("expected action 'login', got %v", tags["action"])
	}
}

func TestSlogHandlerWithAttrs(t *testing.T) {
	h, logger := newTestSlogHandler()
	h2 := h.WithAttrs([]slog.Attr{slog.String("env", "prod")})

	slogLogger := slog.New(h2)
	slogLogger.Info("with pre-attrs", "key", "value")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	tags := logger.batchQueue[0].Tags
	if tags["env"] != "prod" {
		t.Errorf("expected env 'prod', got %v", tags["env"])
	}
	if tags["key"] != "value" {
		t.Errorf("expected key 'value', got %v", tags["key"])
	}
}

func TestSlogHandlerGroupPrefix(t *testing.T) {
	h, logger := newTestSlogHandler()
	h2 := h.WithGroup("request")

	slogLogger := slog.New(h2)
	slogLogger.Info("grouped", "method", "GET", "path", "/api")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	tags := logger.batchQueue[0].Tags
	if tags["request.method"] != "GET" {
		t.Errorf("expected request.method 'GET', got %v", tags["request.method"])
	}
	if tags["request.path"] != "/api" {
		t.Errorf("expected request.path '/api', got %v", tags["request.path"])
	}
}

func TestSlogHandlerNestedGroups(t *testing.T) {
	h, logger := newTestSlogHandler()
	h2 := h.WithGroup("a").WithGroup("b")

	slogLogger := slog.New(h2)
	slogLogger.Info("nested", "key", "val")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	tags := logger.batchQueue[0].Tags
	if tags["a.b.key"] != "val" {
		t.Errorf("expected a.b.key 'val', got %v", tags["a.b.key"])
	}
}

func TestSlogHandlerEnabled(t *testing.T) {
	h, _ := newTestSlogHandler(WithSlogLevel(slog.LevelWarn))

	ctx := context.Background()

	if h.Enabled(ctx, slog.LevelDebug) {
		t.Error("expected debug to be disabled when level is Warn")
	}
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Error("expected info to be disabled when level is Warn")
	}
	if !h.Enabled(ctx, slog.LevelWarn) {
		t.Error("expected warn to be enabled when level is Warn")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Error("expected error to be enabled when level is Warn")
	}
}

func TestSlogHandlerLevelFiltering(t *testing.T) {
	h, logger := newTestSlogHandler(WithSlogLevel(slog.LevelWarn))
	slogLogger := slog.New(h)

	slogLogger.Debug("debug")
	slogLogger.Info("info")
	slogLogger.Warn("warn")
	slogLogger.Error("error")

	// Only warn and error should be forwarded
	if logger.BatchSize() != 2 {
		t.Fatalf("expected 2 log entries (warn+error), got %d", logger.BatchSize())
	}
	if logger.batchQueue[0].Level != LevelWarn {
		t.Errorf("expected first entry to be warn, got %s", logger.batchQueue[0].Level)
	}
	if logger.batchQueue[1].Level != LevelError {
		t.Errorf("expected second entry to be error, got %s", logger.batchQueue[1].Level)
	}
}

func TestSlogHandlerTruncatesLongMessages(t *testing.T) {
	h, logger := newTestSlogHandler()
	slogLogger := slog.New(h)

	longMsg := strings.Repeat("x", 20000)
	slogLogger.Info(longMsg)

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logger.BatchSize())
	}

	msg := logger.batchQueue[0].Message
	if len(msg) >= 20000 {
		t.Errorf("expected message to be truncated, got len %d", len(msg))
	}
	if !strings.HasSuffix(msg, "... [truncated]") {
		t.Error("expected message to end with '... [truncated]'")
	}
}

func TestSlogHandlerNeverPanics(t *testing.T) {
	// Handler with nil logger — should not panic
	h := &SlogHandler{
		logger: nil,
		level:  slog.LevelDebug,
	}

	// This will panic internally when trying to call nil logger methods,
	// but the recover() in Handle should catch it
	record := slog.Record{}
	record.Message = "test"

	// Should not panic
	err := h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSlogHandlerRecursionGuard(t *testing.T) {
	// Manually test the recursion guard by simulating the sending state
	h, logger := newTestSlogHandler()

	gid := goroutineID()
	slogSending.Store(gid, struct{}{})
	defer slogSending.Delete(gid)

	slogLogger := slog.New(h)
	slogLogger.Info("should be skipped")

	if logger.BatchSize() != 0 {
		t.Errorf("expected 0 log entries due to recursion guard, got %d", logger.BatchSize())
	}
}

func TestGoroutineID(t *testing.T) {
	gid := goroutineID()
	if gid == "" || gid == "0" {
		t.Errorf("expected valid goroutine ID, got '%s'", gid)
	}

	// Should be a numeric string
	for _, c := range gid {
		if c < '0' || c > '9' {
			t.Errorf("expected numeric goroutine ID, got '%s'", gid)
			break
		}
	}
}

func TestSetSlogCapture(t *testing.T) {
	logger := NewLogger("test_key", "test-service")
	logger.BeginBatch()

	// Should not panic
	SetSlogCapture(logger)

	// Verify it was set by logging through slog
	slog.Info("capture test")

	if logger.BatchSize() != 1 {
		t.Fatalf("expected 1 log entry after SetSlogCapture, got %d", logger.BatchSize())
	}
}
