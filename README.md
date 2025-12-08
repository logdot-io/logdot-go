<p align="center">
  <h1 align="center">LogDot SDK for Go</h1>
  <p align="center">
    <strong>Cloud logging and metrics made simple</strong>
  </p>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/logdot-io/logdot-go"><img src="https://img.shields.io/badge/go.dev-reference-blue?style=flat-square&logo=go&logoColor=white" alt="Go Reference"></a>
  <a href="https://github.com/logdot-io/logdot-go/releases"><img src="https://img.shields.io/github/v/release/logdot-io/logdot-go?style=flat-square&color=blue" alt="Release"></a>
  <a href="https://github.com/logdot-io/logdot-go/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="MIT License"></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/go-%3E%3D1.21-blue?style=flat-square&logo=go&logoColor=white" alt="Go 1.21+"></a>
  <a href="https://goreportcard.com/report/github.com/logdot-io/logdot-go"><img src="https://img.shields.io/badge/go_report-A+-brightgreen?style=flat-square" alt="Go Report"></a>
</p>

<p align="center">
  <a href="https://logdot.io">Website</a> •
  <a href="https://docs.logdot.io">Documentation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#api-reference">API Reference</a>
</p>

---

## Features

- **Separate Clients** — Independent logger and metrics clients for maximum flexibility
- **Context-Aware Logging** — Create loggers with persistent context that automatically flows through your application
- **Thread-Safe** — All operations are safe for concurrent use
- **Context Support** — Full `context.Context` support for cancellation and timeouts
- **Entity-Based Metrics** — Create/find entities, then bind to them for organized metric collection
- **Batch Operations** — Efficiently send multiple logs or metrics in a single request
- **Zero Dependencies** — Uses only the Go standard library

## Installation

```bash
go get github.com/logdot-io/logdot-go
```

## Quick Start

```go
package main

import (
    "context"
    logdot "github.com/logdot-io/logdot-go"
)

func main() {
    ctx := context.Background()

    // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
    // LOGGING
    // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
    logger := logdot.NewLogger("ilog_live_YOUR_API_KEY", "my-service")

    logger.Info(ctx, "Application started", nil)
    logger.Error(ctx, "Something went wrong", map[string]interface{}{
        "error_code": 500,
    })

    // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
    // METRICS
    // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
    metrics := logdot.NewMetrics("ilog_live_YOUR_API_KEY")

    // Create or find an entity first
    entity, _ := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{
        Name:        "my-service",
        Description: "My production service",
    })

    // Bind to the entity for sending metrics
    metricsClient := metrics.ForEntity(entity.ID)
    metricsClient.Send(ctx, "response_time", 123.45, "ms", nil)
}
```

## Logging

### Configuration

```go
logger := logdot.NewLogger("ilog_live_YOUR_API_KEY", "my-service",
    logdot.WithLoggerTimeout(5*time.Second),
    logdot.WithLoggerRetry(3, time.Second, 30*time.Second),
    logdot.WithLoggerDebug(true),
)
```

### Log Levels

```go
logger.Debug(ctx, "Debug message", nil)
logger.Info(ctx, "Info message", nil)
logger.Warn(ctx, "Warning message", nil)
logger.Error(ctx, "Error message", nil)
```

### Structured Tags

```go
logger.Info(ctx, "User logged in", map[string]interface{}{
    "user_id":    12345,
    "ip_address": "192.168.1.1",
    "browser":    "Chrome",
})
```

### Context-Aware Logging

Create loggers with persistent context that automatically flows through your application:

```go
// Create a logger with context for a specific request
requestLogger := logger.WithContext(map[string]interface{}{
    "request_id": "abc-123",
    "user_id":    456,
})

// All logs include request_id and user_id automatically
requestLogger.Info(ctx, "Processing request", nil)
requestLogger.Debug(ctx, "Fetching user data", nil)

// Chain contexts — they merge together
detailedLogger := requestLogger.WithContext(map[string]interface{}{
    "operation": "checkout",
})

// This log has request_id, user_id, AND operation
detailedLogger.Info(ctx, "Starting checkout process", nil)
```

### Batch Logging

Send multiple logs in a single HTTP request:

```go
logger.BeginBatch()

logger.Info(ctx, "Step 1 complete", nil)
logger.Info(ctx, "Step 2 complete", nil)
logger.Info(ctx, "Step 3 complete", nil)

logger.SendBatch(ctx)  // Single HTTP request
logger.EndBatch()
```

## Metrics

### Entity Management

```go
metrics := logdot.NewMetrics("...")

// Create a new entity
entity, err := metrics.CreateEntity(ctx, logdot.CreateEntityOptions{
    Name:        "my-service",
    Description: "Production API server",
    Metadata:    map[string]interface{}{"environment": "production"},
})

// Find existing entity
existing, err := metrics.GetEntityByName(ctx, "my-service")

// Get or create (recommended)
entity, err := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{
    Name:        "my-service",
    Description: "Created if not exists",
})
```

### Sending Metrics

```go
metricsClient := metrics.ForEntity(entity.ID)

// Single metric
metricsClient.Send(ctx, "cpu_usage", 45.2, "percent", nil)
metricsClient.Send(ctx, "response_time", 123.45, "ms", map[string]interface{}{
    "endpoint": "/api/users",
    "method":   "GET",
})
```

### Batch Metrics

```go
// Same metric, multiple values
metricsClient.BeginBatch("temperature", "celsius")
metricsClient.Add(23.5, nil)
metricsClient.Add(24.1, nil)
metricsClient.Add(23.8, nil)
metricsClient.SendBatch(ctx)
metricsClient.EndBatch()

// Multiple different metrics
metricsClient.BeginMultiBatch()
metricsClient.AddMetric("cpu_usage", 45.2, "percent", nil)
metricsClient.AddMetric("memory_used", 2048, "MB", nil)
metricsClient.AddMetric("disk_free", 50.5, "GB", nil)
metricsClient.SendBatch(ctx)
metricsClient.EndBatch()
```

## API Reference

### Logger

| Method | Description |
|--------|-------------|
| `WithContext(context)` | Create new logger with merged context |
| `GetContext()` | Get current context map |
| `Debug/Info/Warn/Error(ctx, message, tags)` | Send log at level |
| `BeginBatch()` | Start batch mode |
| `SendBatch(ctx)` | Send queued logs |
| `EndBatch()` | End batch mode |
| `ClearBatch()` | Clear queue without sending |
| `BatchSize()` | Get queue size |

### Metrics

| Method | Description |
|--------|-------------|
| `CreateEntity(ctx, options)` | Create a new entity |
| `GetEntityByName(ctx, name)` | Find entity by name |
| `GetOrCreateEntity(ctx, options)` | Get existing or create new |
| `ForEntity(entityId)` | Create bound metrics client |

### BoundMetrics

| Method | Description |
|--------|-------------|
| `Send(ctx, name, value, unit, tags)` | Send single metric |
| `BeginBatch(name, unit)` | Start single-metric batch |
| `Add(value, tags)` | Add to batch |
| `BeginMultiBatch()` | Start multi-metric batch |
| `AddMetric(name, value, unit, tags)` | Add metric to batch |
| `SendBatch(ctx)` | Send queued metrics |
| `EndBatch()` | End batch mode |

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <a href="https://logdot.io">logdot.io</a> •
  Built with care for developers
</p>
