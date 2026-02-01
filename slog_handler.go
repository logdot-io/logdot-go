package logdot

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
)

// SlogHandler is a slog.Handler that forwards structured log records to LogDot.
//
// It maps slog levels to LogDot severities, flattens attributes into tags,
// and includes a goroutine-based recursion guard to prevent infinite loops
// when LogDot's HTTP client triggers slog output.
//
// Example:
//
//	logger := logdot.NewLogger("ilog_live_xxx", "my-service")
//	slog.SetDefault(slog.New(logdot.NewSlogHandler(logger)))
//
//	slog.Info("hello", "key", "value")  // forwarded to LogDot
type SlogHandler struct {
	logger *Logger
	level  slog.Leveler
	attrs  []slog.Attr
	group  string
}

// SlogHandlerOption configures a SlogHandler.
type SlogHandlerOption func(*SlogHandler)

// WithSlogLevel sets the minimum slog level that will be forwarded.
// Defaults to slog.LevelDebug (all levels forwarded).
func WithSlogLevel(level slog.Leveler) SlogHandlerOption {
	return func(h *SlogHandler) {
		h.level = level
	}
}

// NewSlogHandler creates a slog.Handler that forwards records to LogDot.
//
// Example:
//
//	h := logdot.NewSlogHandler(logger, logdot.WithSlogLevel(slog.LevelInfo))
//	slog.SetDefault(slog.New(h))
func NewSlogHandler(logger *Logger, opts ...SlogHandlerOption) *SlogHandler {
	h := &SlogHandler{
		logger: logger,
		level:  slog.LevelDebug,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Enabled reports whether the handler handles records at the given level.
func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle processes a log record by forwarding it to LogDot.
func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Goroutine-based recursion guard: prevent LogDot's HTTP calls
	// from triggering slog → LogDot → slog infinite loops.
	gid := goroutineID()
	if _, loaded := slogSending.LoadOrStore(gid, struct{}{}); loaded {
		return nil
	}
	defer slogSending.Delete(gid)

	defer func() { recover() }() //nolint:errcheck // never crash

	message := truncateMessage(record.Message)
	level := mapSlogLevel(record.Level)

	tags := make(map[string]interface{})
	tags["source"] = "slog"

	// Add pre-configured attrs
	for _, attr := range h.attrs {
		h.addAttr(tags, h.group, attr)
	}

	// Add record attrs
	record.Attrs(func(a slog.Attr) bool {
		h.addAttr(tags, h.group, a)
		return true
	})

	bgCtx := context.Background()
	switch level {
	case LevelDebug:
		h.logger.Debug(bgCtx, message, tags)
	case LevelWarn:
		h.logger.Warn(bgCtx, message, tags)
	case LevelError:
		h.logger.Error(bgCtx, message, tags)
	default:
		h.logger.Info(bgCtx, message, tags)
	}

	return nil
}

// WithAttrs returns a new handler with the given attributes added.
func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	newAttrs = append(newAttrs, attrs...)

	return &SlogHandler{
		logger: h.logger,
		level:  h.level,
		attrs:  newAttrs,
		group:  h.group,
	}
}

// WithGroup returns a new handler with the given group name set.
// Attributes added via WithAttrs or from records will be prefixed with
// the group name using dot notation (e.g. "group.key").
func (h *SlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}

	newAttrs := make([]slog.Attr, len(h.attrs))
	copy(newAttrs, h.attrs)

	return &SlogHandler{
		logger: h.logger,
		level:  h.level,
		attrs:  newAttrs,
		group:  newGroup,
	}
}

// addAttr adds a single slog.Attr to the tags map with optional group prefix.
func (h *SlogHandler) addAttr(tags map[string]interface{}, prefix string, a slog.Attr) {
	val := a.Value.Resolve()

	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}

	if val.Kind() == slog.KindGroup {
		for _, ga := range val.Group() {
			h.addAttr(tags, key, ga)
		}
		return
	}

	tags[key] = val.Any()
}

// mapSlogLevel converts a slog.Level to a LogDot LogLevel.
func mapSlogLevel(level slog.Level) LogLevel {
	switch {
	case level >= slog.LevelError:
		return LevelError
	case level >= slog.LevelWarn:
		return LevelWarn
	case level >= slog.LevelInfo:
		return LevelInfo
	default:
		return LevelDebug
	}
}

// slogSending tracks which goroutines are currently inside the handler
// to prevent recursion. Keys are goroutine ID strings.
var slogSending sync.Map

// goroutineID returns the current goroutine's ID as a string.
// This is intentionally kept simple — it parses the goroutine ID from
// runtime.Stack() output which always starts with "goroutine NNN [".
func goroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Output starts with "goroutine NNN ["
	s := string(buf[:n])
	// Skip "goroutine "
	const prefix = "goroutine "
	if len(s) < len(prefix) {
		return "0"
	}
	s = s[len(prefix):]
	// Read digits
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return s[:i]
		}
	}
	return "0"
}

// Verify interface compliance at compile time.
var _ slog.Handler = (*SlogHandler)(nil)

// SetSlogCapture is a convenience function that installs a SlogHandler
// as the default slog handler.
//
// Example:
//
//	logdot.SetSlogCapture(logger)
//	slog.Info("this goes to LogDot")
func SetSlogCapture(logger *Logger, opts ...SlogHandlerOption) {
	slog.SetDefault(slog.New(NewSlogHandler(logger, opts...)))
}

