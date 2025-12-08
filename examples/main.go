// LogDot SDK Test Application
//
// This script tests all SDK functionality against the live LogDot API.
//
// Setup: Create a .env file in the project root with:
//
//	LOGDOT_API_KEY=ilog_live_YOUR_API_KEY
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	logdot "github.com/logdot-io/logdot-go"
)

func loadEnv() error {
	// Try to find .env file in parent directories
	envPath := filepath.Join("..", "..", ".env")
	file, err := os.Open(envPath)
	if err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	return scanner.Err()
}

var apiKey string

func sleep(d time.Duration) {
	time.Sleep(d)
}

func printSummary(passed, failed int) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Total:  %d\n", passed+failed)
	fmt.Printf("  Passed: %d\n", passed)
	fmt.Printf("  Failed: %d\n", failed)
	fmt.Println(strings.Repeat("=", 60))

	if failed > 0 {
		fmt.Println("\nSome tests failed. Check the output above for details.")
		os.Exit(1)
	} else {
		fmt.Println("\nAll tests passed! The Go SDK is working correctly.")
		os.Exit(0)
	}
}

func main() {
	// Load environment variables from .env file
	if err := loadEnv(); err != nil {
		fmt.Println("Failed to load .env file. Create one with LOGDOT_API_KEY=your_key")
		os.Exit(1)
	}

	apiKey = os.Getenv("LOGDOT_API_KEY")
	if apiKey == "" {
		fmt.Println("LOGDOT_API_KEY not found in .env file")
		os.Exit(1)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("LogDot Go SDK Test Application")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	ctx := context.Background()

	// Create separate logger and metrics clients
	logger := logdot.NewLogger(apiKey, "go-test-app",
		logdot.WithLoggerDebug(true),
	)

	metrics := logdot.NewMetrics(apiKey,
		logdot.WithMetricsDebug(true),
	)

	passed := 0
	failed := 0

	// ==================== Test 1: Single Logs ====================
	fmt.Println("\n--- Test 1: Single Logs (all levels) ---\n")

	// Debug
	if err := logger.Debug(ctx, "Test debug message from Go SDK", nil); err == nil {
		fmt.Println("  [PASS] debug log sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] debug log failed: %v\n", err)
		failed++
	}
	sleep(500 * time.Millisecond)

	// Info
	if err := logger.Info(ctx, "Test info message from Go SDK", nil); err == nil {
		fmt.Println("  [PASS] info log sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] info log failed: %v\n", err)
		failed++
	}
	sleep(500 * time.Millisecond)

	// Warn
	if err := logger.Warn(ctx, "Test warn message from Go SDK", nil); err == nil {
		fmt.Println("  [PASS] warn log sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] warn log failed: %v\n", err)
		failed++
	}
	sleep(500 * time.Millisecond)

	// Error
	if err := logger.Error(ctx, "Test error message from Go SDK", nil); err == nil {
		fmt.Println("  [PASS] error log sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] error log failed: %v\n", err)
		failed++
	}
	sleep(500 * time.Millisecond)

	// ==================== Test 2: Logs with Tags ====================
	fmt.Println("\n--- Test 2: Logs with Tags ---\n")

	tags := map[string]interface{}{
		"sdk":       "go",
		"version":   "1.0.0",
		"test":      true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if err := logger.Info(ctx, "Log with structured tags", tags); err == nil {
		fmt.Println("  [PASS] Log with tags sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] Log with tags failed: %v\n", err)
		failed++
	}
	sleep(500 * time.Millisecond)

	// ==================== Test 3: Context-aware Logging ====================
	fmt.Println("\n--- Test 3: Context-aware Logging ---\n")

	userLogger := logger.WithContext(map[string]interface{}{
		"user_id": 123,
		"session": "abc-123",
	})
	if err := userLogger.Info(ctx, "User performed action", map[string]interface{}{"action": "login"}); err == nil {
		fmt.Println("  [PASS] Context-aware log sent successfully")
		fmt.Printf("  Context: %v\n", userLogger.GetContext())
		passed++
	} else {
		fmt.Printf("  [FAIL] Context-aware log failed: %v\n", err)
		failed++
	}
	sleep(500 * time.Millisecond)

	// ==================== Test 4: Chained Context ====================
	fmt.Println("\n--- Test 4: Chained Context ---\n")

	// Chain contexts - add more context to existing context
	requestLogger := userLogger.WithContext(map[string]interface{}{
		"request_id": "req-456",
		"endpoint":   "/api/users",
	})
	if err := requestLogger.Info(ctx, "Processing request", nil); err == nil {
		fmt.Println("  [PASS] Chained context log sent successfully")
		fmt.Printf("  Original context: %v\n", userLogger.GetContext())
		fmt.Printf("  Chained context: %v\n", requestLogger.GetContext())
		passed++
	} else {
		fmt.Printf("  [FAIL] Chained context log failed: %v\n", err)
		failed++
	}

	// Test context value overwriting
	overwriteLogger := userLogger.WithContext(map[string]interface{}{"user_id": 456})
	fmt.Printf("  Overwrite test - new user_id: %v\n", overwriteLogger.GetContext()["user_id"])
	sleep(500 * time.Millisecond)

	// ==================== Test 5: Batch Logs ====================
	fmt.Println("\n--- Test 5: Batch Logs (all levels) ---\n")

	logger.BeginBatch()
	fmt.Println("  Started batch mode")

	// Test all severity levels in batch
	logger.Debug(ctx, "Batch debug message", map[string]interface{}{"level": "debug"})
	logger.Info(ctx, "Batch info message", map[string]interface{}{"level": "info"})
	logger.Warn(ctx, "Batch warn message", map[string]interface{}{"level": "warn"})
	logger.Error(ctx, "Batch error message", map[string]interface{}{"level": "error"})
	fmt.Printf("  Added 4 messages (all levels) to batch (size: %d)\n", logger.BatchSize())

	if err := logger.SendBatch(ctx); err == nil {
		fmt.Println("  [PASS] Batch logs sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] Batch logs failed: %v\n", err)
		failed++
	}

	logger.EndBatch()
	fmt.Println("  Ended batch mode")
	sleep(500 * time.Millisecond)

	// ==================== Test 6: Clear Batch ====================
	fmt.Println("\n--- Test 6: Clear Batch ---\n")

	logger.BeginBatch()
	logger.Info(ctx, "This message will be cleared", nil)
	logger.Info(ctx, "This one too", nil)
	fmt.Printf("  Batch size before clear: %d\n", logger.BatchSize())

	logger.ClearBatch()
	fmt.Printf("  Batch size after clear: %d\n", logger.BatchSize())

	if logger.BatchSize() == 0 {
		fmt.Println("  [PASS] ClearBatch works correctly")
		passed++
	} else {
		fmt.Println("  [FAIL] ClearBatch did not clear the batch")
		failed++
	}

	logger.EndBatch()
	sleep(500 * time.Millisecond)

	// ==================== Test 7: Create/Get Metrics Entity ====================
	fmt.Println("\n--- Test 7: Create/Get Metrics Entity ---\n")

	metadata := map[string]interface{}{
		"sdk":         "go",
		"environment": "test",
		"created_at":  time.Now().UTC().Format(time.RFC3339),
	}
	entity, err := metrics.GetOrCreateEntity(ctx, logdot.CreateEntityOptions{
		Name:        "go-test-entity",
		Description: "Go SDK Test Entity",
		Metadata:    metadata,
	})

	if err == nil && entity != nil {
		fmt.Printf("  [PASS] Entity created/found (ID: %s)\n", entity.ID)
		passed++
	} else {
		fmt.Printf("  [FAIL] Entity creation failed: %v\n", err)
		failed++
		// Cannot continue without entity
		printSummary(passed, failed)
		return
	}
	sleep(500 * time.Millisecond)

	// ==================== Test 8: Single Metrics (using ForEntity) ====================
	fmt.Println("\n--- Test 8: Single Metrics ---\n")

	metricsClient := metrics.ForEntity(entity.ID)

	metricTags := map[string]interface{}{
		"host": "go-test",
		"core": 0,
	}
	if err := metricsClient.Send(ctx, "cpu_usage", 45.5, "percent", metricTags); err == nil {
		fmt.Println("  [PASS] Single metric sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] Single metric failed: %v\n", err)
		failed++
	}
	sleep(500 * time.Millisecond)

	// ==================== Test 9: Batch Metrics (Same Metric) ====================
	fmt.Println("\n--- Test 9: Batch Metrics (Same Metric) ---\n")

	metricsClient.BeginBatch("temperature", "celsius")
	fmt.Println("  Started batch mode for \"temperature\"")

	temperatures := []float64{23.5, 24.1, 23.8, 24.5, 25.0}
	for _, temp := range temperatures {
		metricsClient.Add(temp, map[string]interface{}{"location": "server_room"})
	}
	fmt.Printf("  Added %d values (size: %d)\n", len(temperatures), metricsClient.BatchSize())

	if err := metricsClient.SendBatch(ctx); err == nil {
		fmt.Println("  [PASS] Metric batch sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] Metric batch failed: %v\n", err)
		failed++
	}

	metricsClient.EndBatch()
	fmt.Println("  Ended batch mode")
	sleep(500 * time.Millisecond)

	// ==================== Test 10: Multi-Metric Batch ====================
	fmt.Println("\n--- Test 10: Multi-Metric Batch ---\n")

	metricsClient.BeginMultiBatch()
	fmt.Println("  Started multi-metric batch mode")

	metricsClient.AddMetric("memory_used", 2048, "MB", map[string]interface{}{"type": "heap"})
	metricsClient.AddMetric("disk_free", 50.5, "GB", map[string]interface{}{"mount": "/"})
	metricsClient.AddMetric("network_latency", 12.3, "ms", map[string]interface{}{"interface": "eth0"})
	fmt.Printf("  Added 3 different metrics (size: %d)\n", metricsClient.BatchSize())

	if err := metricsClient.SendBatch(ctx); err == nil {
		fmt.Println("  [PASS] Multi-metric batch sent successfully")
		passed++
	} else {
		fmt.Printf("  [FAIL] Multi-metric batch failed: %v\n", err)
		failed++
	}

	metricsClient.EndBatch()
	fmt.Println("  Ended batch mode")

	// ==================== Summary ====================
	printSummary(passed, failed)
}
