# LogDot SDK for Go

Official Go SDK for [LogDot](https://logdot.io) - Cloud logging and metrics made simple.

## Features

- **Separate Clients**: Independent logger and metrics clients for flexibility
- **Context-Aware Logging**: Create loggers with persistent context that's automatically added to all logs
- **Thread-Safe**: All operations are safe for concurrent use
- **Context Support**: Full context.Context support for cancellation and timeouts
- **Flexible Logging**: 4 log levels (debug, info, warn, error) with structured tags
- **Entity-Based Metrics**: Create/find entities, then bind to them for sending metrics
- **Batch Operations**: Efficiently send multiple logs or metrics in a single request
- **Automatic Retry**: Exponential backoff retry with configurable attempts
- **Zero Dependencies**: Uses only the Go standard library

## Installation

```bash
go get github.com/logdot/logdot-go
```

## Quick Start

```go
package main

import (
    "context"
    logdot "github.com/logdot/logdot-go"
)

func main() {
    ctx := context.Background()

    // === LOGGING ===
    logger := logdot.NewLogger("ilog_live_YOUR_API_KEY", "my-service")

    logger.Info(ctx, "Application started", nil)
    logger.Error(ctx, "Something went wrong", map[string]interface{}{
        "error_code": 500,
    })

    // === METRICS ===
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
import logdot "github.com/logdot/logdot-go"

logger := logdot.NewLogger("ilog_live_YOUR_API_KEY", "my-service",
    logdot.WithLoggerTimeout(5*time.Second),     // HTTP timeout
    logdot.WithLoggerRetry(3, time.Second, 30*time.Second), // Retry config
    logdot.WithLoggerDebug(true),                // Enable debug output
)
```

### Basic Logging

```go
ctx := context.Background()

logger.Debug(ctx, "Debug message", nil)
logger.Info(ctx, "Info message", nil)
logger.Warn(ctx, "Warning message", nil)
logger.Error(ctx, "Error message", nil)
```

### Logging with Tags

```go
logger.Info(ctx, "User logged in", map[string]interface{}{
    "user_id":    12345,
    "ip_address": "192.168.1.1",
    "browser":    "Chrome",
})

logger.Error(ctx, "Database connection failed", map[string]interface{}{
    "host":  "db.example.com",
    "port":  5432,
    "error": "Connection timeout",
})
```

### Context-Aware Logging

Create loggers with persistent context that's automatically added to all logs:

```go
logger := logdot.NewLogger("ilog_live_YOUR_API_KEY", "my-service")

// Create a logger with context for a specific request
requestLogger := logger.WithContext(map[string]interface{}{
    "request_id": "abc-123",
    "user_id":    456,
})

// All logs from requestLogger will include request_id and user_id
requestLogger.Info(ctx, "Processing request", nil)
requestLogger.Debug(ctx, "Fetching user data", nil)
requestLogger.Info(ctx, "Request completed", nil)

// You can chain contexts - they merge together
detailedLogger := requestLogger.WithContext(map[string]interface{}{
    "operation": "checkout",
})

// This log will have request_id, user_id, AND operation
detailedLogger.Info(ctx, "Starting checkout process", nil)

// Original logger is unchanged
logger.Info(ctx, "This log has no context", nil)
```

### Context with Additional Tags

When you provide tags to a log call, they're merged with the context (tags take precedence):

```go
logger := logdot.NewLogger("...", "api").WithContext(map[string]interface{}{
    "service":     "api",
    "environment": "production",
})

// The log will have: service, environment, endpoint, status
logger.Info(ctx, "Request handled", map[string]interface{}{
    "endpoint": "/users",
    "status":   200,
})

// Override context values if needed
logger.Info(ctx, "Custom service", map[string]interface{}{
    "service": "worker", // This overrides the context value
})
```

### Batch Logging

Send multiple logs in a single HTTP request:

```go
// Start batch mode
logger.BeginBatch()

// Queue logs (no network calls yet)
logger.Info(ctx, "Request received", nil)
logger.Debug(ctx, "Processing started", nil)
logger.Info(ctx, "Processing complete", nil)

// Send all logs in one request
logger.SendBatch(ctx)

// End batch mode
logger.EndBatch()
```

## Metrics

### Configuration

```go
import logdot "github.com/logdot/logdot-go"

metrics := logdot.NewMetrics("ilog_live_YOUR_API_KEY",
    logdot.WithMetricsTimeout(5*time.Second),     // HTTP timeout
    logdot.WithMetricsRetry(3, time.Second, 30*time.Second), // Retry config
    logdot.WithMetricsDebug(true),                // Enable debug output
)
```

### Entity Management

Before sending metrics, you need to create or find an entity:

```go
// Create a new entity
entity, err := metrics.CreateEntity(ctx, logdot.CreateEntityOptions{
    Name:        "my-service",
    Description: "My production service",
    Metadata: map[string]interface{}{
        "environment": "production",
        "region":      "us-east-1",
        "version":     "1.2.3",
    },
})

// Or find an existing entity by name
existing, err := metrics.GetEntityByName(ctx, "my-service")

// Or get or create (finds existing, creates if not found)
entity, err := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{
    Name:        "my-service",
    Description: "Created if not exists",
})
```

### Binding to an Entity

Once you have an entity, bind to it for sending metrics:

```go
entity, _ := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{Name: "my-service"})
metricsClient := metrics.ForEntity(entity.ID)

// Now send metrics
metricsClient.Send(ctx, "cpu_usage", 45.2, "percent", nil)
metricsClient.Send(ctx, "response_time", 123.45, "ms", map[string]interface{}{
    "endpoint": "/api/users",
    "method":   "GET",
})
```

### Batch Metrics (Same Metric)

Send multiple values for the same metric:

```go
// Start batch for a specific metric
metricsClient.BeginBatch("temperature", "celsius")

// Add values
metricsClient.Add(23.5, nil)
metricsClient.Add(24.1, nil)
metricsClient.Add(23.8, nil)
metricsClient.Add(24.5, nil)

// Send all values in one request
metricsClient.SendBatch(ctx)

// End batch mode
metricsClient.EndBatch()
```

### Multi-Metric Batch

Send different metrics in a single request:

```go
// Start multi-metric batch
metricsClient.BeginMultiBatch()

// Add different metrics
metricsClient.AddMetric("cpu_usage", 45.2, "percent", nil)
metricsClient.AddMetric("memory_used", 2048, "MB", nil)
metricsClient.AddMetric("disk_free", 50.5, "GB", nil)

// Send all metrics in one request
metricsClient.SendBatch(ctx)

// End batch mode
metricsClient.EndBatch()
```

## Error Handling

```go
// Check errors from operations
if err := logger.Info(ctx, "Test message", nil); err != nil {
    log.Printf("Failed to send log: %v", err)
}

// For metrics
if err := metricsClient.Send(ctx, "test", 1, "unit", nil); err != nil {
    log.Printf("Failed to send metric: %v", err)
    log.Printf("Last error: %s", metricsClient.LastError())
    log.Printf("HTTP code: %d", metricsClient.LastHTTPCode())
}
```

## Debug Mode

Enable debug output to see HTTP requests and responses:

```go
logger := logdot.NewLogger("...", "my-service",
    logdot.WithLoggerDebug(true), // Enable at construction
)

// Or enable later
logger.SetDebug(true)
```

## API Reference

### Logger (from `NewLogger`)

| Method | Description |
|--------|-------------|
| `WithContext(context)` | Create new logger with merged context |
| `GetContext()` | Get current context map |
| `Debug(ctx, message, tags)` | Send debug log |
| `Info(ctx, message, tags)` | Send info log |
| `Warn(ctx, message, tags)` | Send warning log |
| `Error(ctx, message, tags)` | Send error log |
| `Log(ctx, level, message, tags)` | Send log at level |
| `BeginBatch()` | Start batch mode |
| `SendBatch(ctx)` | Send queued logs |
| `EndBatch()` | End batch mode |
| `ClearBatch()` | Clear queue |
| `BatchSize()` | Get queue size |
| `Hostname()` | Get hostname |
| `SetDebug(enabled)` | Enable/disable debug |

### Metrics (from `NewMetrics`)

| Method | Description |
|--------|-------------|
| `CreateEntity(ctx, options)` | Create a new entity |
| `GetEntityByName(ctx, name)` | Find entity by name |
| `GetOrCreateEntity(ctx, options)` | Get existing or create new entity |
| `ForEntity(entityId)` | Create bound client for entity |
| `LastError()` | Get last error message |
| `LastHTTPCode()` | Get last HTTP code |
| `SetDebug(enabled)` | Enable/disable debug |

### BoundMetrics (from `ForEntity`)

| Method | Description |
|--------|-------------|
| `EntityID()` | Get bound entity ID |
| `Send(ctx, name, value, unit, tags)` | Send single metric |
| `BeginBatch(name, unit)` | Start single-metric batch |
| `Add(value, tags)` | Add to single-metric batch |
| `BeginMultiBatch()` | Start multi-metric batch |
| `AddMetric(name, value, unit, tags)` | Add to multi-metric batch |
| `SendBatch(ctx)` | Send queued metrics |
| `EndBatch()` | End batch mode |
| `ClearBatch()` | Clear queue |
| `BatchSize()` | Get queue size |
| `LastError()` | Get last error message |
| `LastHTTPCode()` | Get HTTP code |
| `SetDebug(enabled)` | Enable/disable debug |

## Requirements

- Go 1.21 or higher

## License

MIT License - see [LICENSE](LICENSE) for details.

## Links

- [LogDot Website](https://logdot.io)
- [Documentation](https://docs.logdot.io/go)
- [GitHub Repository](https://github.com/logdot/logdot-go)
