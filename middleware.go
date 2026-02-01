package logdot

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"
)

const maxMessageBytes = 16000

// MiddlewareConfig configures the HTTP auto-instrumentation middleware.
type MiddlewareConfig struct {
	// Logger is required — all requests are logged through this instance.
	Logger *Logger

	// Metrics is optional. When set together with LogMetrics, request
	// duration metrics are sent to LogDot.
	Metrics *Metrics

	// EntityName is used for lazy entity resolution. Defaults to
	// Logger.Hostname() when empty.
	EntityName string

	// LogRequests enables per-request log entries.
	// Zero value (false) means the caller must explicitly set it to true.
	// Use DefaultMiddlewareConfig() for sane defaults.
	LogRequests bool

	// LogMetrics enables sending http.request.duration metrics.
	LogMetrics bool

	// IgnorePaths lists URL paths that should not be logged or metered.
	IgnorePaths []string
}

// DefaultMiddlewareConfig returns a MiddlewareConfig with sensible defaults.
// Logger and Metrics still need to be set by the caller.
func DefaultMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		LogRequests: true,
		LogMetrics:  true,
	}
}

// Middleware returns an http.Handler middleware that automatically logs
// requests and sends duration metrics to LogDot.
//
// Compatible with net/http, Chi, Gorilla, and any router that uses
// the standard http.Handler interface.
//
// Example:
//
//	cfg := logdot.DefaultMiddlewareConfig()
//	cfg.Logger = logger
//	cfg.Metrics = metrics
//	cfg.EntityName = "my-api"
//
//	handler := logdot.Middleware(cfg)(mux)
//	http.ListenAndServe(":8080", handler)
func Middleware(config MiddlewareConfig) func(http.Handler) http.Handler {
	ignorePaths := make(map[string]struct{}, len(config.IgnorePaths))
	for _, p := range config.IgnorePaths {
		ignorePaths[p] = struct{}{}
	}

	entityName := config.EntityName
	if entityName == "" && config.Logger != nil {
		entityName = config.Logger.Hostname()
	}

	mw := &middlewareState{
		config:      config,
		ignorePaths: ignorePaths,
		entityName:  entityName,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Never break request handling
			defer func() {
				if rec := recover(); rec != nil {
					// If the inner handler panicked, return 500
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			// Skip ignored paths
			if _, skip := mw.ignorePaths[r.URL.Path]; skip {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			durationMs := float64(time.Since(start).Microseconds()) / 1000.0

			if config.LogRequests && config.Logger != nil {
				mw.logRequest(r, rec.status, durationMs)
			}

			if config.LogMetrics && config.Metrics != nil {
				mw.sendMetric(r, rec.status, durationMs)
			}
		})
	}
}

// middlewareState holds the shared state for the middleware closure.
type middlewareState struct {
	config      MiddlewareConfig
	ignorePaths map[string]struct{}
	entityName  string

	entityMu     sync.Mutex
	entityDone   bool
	boundMetrics *BoundMetrics
}

func (mw *middlewareState) logRequest(r *http.Request, status int, durationMs float64) {
	defer func() { recover() }() //nolint:errcheck // never crash

	method := r.Method
	path := r.URL.Path
	message := truncateMessage(fmt.Sprintf("%s %s %d (%.0fms)", method, path, status, durationMs))

	tags := map[string]interface{}{
		"http_method": method,
		"http_path":   path,
		"http_status": status,
		"duration_ms": round2(durationMs),
		"source":      "http_middleware",
	}

	level := severityFromStatus(status)

	// Use background context — logging should not be tied to client's request ctx
	ctx := context.Background()
	switch level {
	case LevelError:
		mw.config.Logger.Error(ctx, message, tags)
	case LevelWarn:
		mw.config.Logger.Warn(ctx, message, tags)
	default:
		mw.config.Logger.Info(ctx, message, tags)
	}
}

func (mw *middlewareState) sendMetric(r *http.Request, status int, durationMs float64) {
	defer func() { recover() }() //nolint:errcheck // never crash

	mw.ensureEntity()

	if mw.boundMetrics == nil {
		return
	}

	mw.boundMetrics.Send(
		context.Background(),
		"http.request.duration",
		round2(durationMs),
		"ms",
		map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
			"status": fmt.Sprintf("%d", status),
		},
	)
}

func (mw *middlewareState) ensureEntity() {
	mw.entityMu.Lock()
	defer mw.entityMu.Unlock()

	if mw.entityDone {
		return
	}

	entity, err := mw.config.Metrics.GetOrCreateEntity(
		context.Background(),
		CreateEntityOptions{
			Name:        mw.entityName,
			Description: fmt.Sprintf("HTTP service: %s", mw.entityName),
		},
	)
	if err == nil && entity != nil {
		mw.boundMetrics = mw.config.Metrics.ForEntity(entity.ID)
		mw.entityDone = true
	}
	// On failure, entityDone stays false so next request retries
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
		// status stays at default 200
	}
	return r.ResponseWriter.Write(b)
}

// --- helpers ---

func severityFromStatus(status int) LogLevel {
	switch {
	case status >= 500:
		return LevelError
	case status >= 400:
		return LevelWarn
	default:
		return LevelInfo
	}
}

func truncateMessage(msg string) string {
	if len(msg) <= maxMessageBytes {
		return msg
	}
	// Walk back to a valid UTF-8 boundary
	truncated := msg[:maxMessageBytes]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "... [truncated]"
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
