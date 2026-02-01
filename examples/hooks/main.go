// LogDot SDK Hooks Test Application
//
// Tests the HTTP middleware and slog handler against the live LogDot API.
// Starts an HTTP server on a random port, makes self-requests, and reports results.
//
// Setup: Create a .env file in the project root with:
//
//	LOGDOT_API_KEY=ilog_live_YOUR_API_KEY
//
// Run: go run main.go (from this directory)
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	logdot "github.com/logdot-io/logdot-go"
)

func loadEnv() error {
	envPath := filepath.Join("..", "..", "..", ".env")
	file, err := os.Open(envPath)
	if err != nil {
		return fmt.Errorf("failed to load .env file at %s: %w", envPath, err)
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

var (
	passed int
	failed int
)

func pass(msg string) {
	fmt.Printf("  [PASS] %s\n", msg)
	passed++
}

func fail(msg string) {
	fmt.Printf("  [FAIL] %s\n", msg)
	failed++
}

func sleep() {
	time.Sleep(500 * time.Millisecond)
}

func main() {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("LogDot Go Hooks Test Application")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	if err := loadEnv(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	apiKey := os.Getenv("LOGDOT_API_KEY")
	if apiKey == "" {
		fmt.Println("LOGDOT_API_KEY not found in .env file")
		os.Exit(1)
	}

	// Create SDK clients
	logger := logdot.NewLogger(apiKey, "go-hooks-test", logdot.WithLoggerDebug(true))
	metrics := logdot.NewMetrics(apiKey, logdot.WithMetricsDebug(true))

	// ==================== Set up HTTP server with middleware ====================

	mux := http.NewServeMux()

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users":[]}`))
	})

	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	mux.HandleFunc("/not-found", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic from hooks test")
	})

	cfg := logdot.DefaultMiddlewareConfig()
	cfg.Logger = logger
	cfg.Metrics = metrics
	cfg.EntityName = "go-hooks-test"
	cfg.IgnorePaths = []string{"/health"}

	handler := logdot.Middleware(cfg)(mux)

	// Start server on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	fmt.Printf("  Server listening on port %d\n", port)

	server := &http.Server{Handler: handler}
	go server.Serve(listener)

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// ==================== Test 1: Middleware 2xx ====================
	fmt.Println("\n--- Test 1: Middleware logs GET 200 as info ---\n")
	testRequest("GET", baseURL+"/api/users", 200, "GET /api/users returned 200 (logged as info)")
	sleep()

	// ==================== Test 2: Middleware 4xx ====================
	fmt.Println("\n--- Test 2: Middleware logs GET 404 as warn ---\n")
	testRequest("GET", baseURL+"/not-found", 404, "GET /not-found returned 404 (logged as warn)")
	sleep()

	// ==================== Test 3: Middleware 5xx ====================
	fmt.Println("\n--- Test 3: Middleware logs GET 500 as error ---\n")
	testRequest("GET", baseURL+"/api/error", 500, "GET /api/error returned 500 (logged as error)")
	sleep()

	// ==================== Test 4: POST request ====================
	fmt.Println("\n--- Test 4: POST request ---\n")
	testRequest("POST", baseURL+"/api/users", 200, "POST /api/users returned 200 (logged with method=POST)")
	sleep()

	// ==================== Test 5: Ignored path ====================
	fmt.Println("\n--- Test 5: Ignored path (no logging) ---\n")
	testRequest("GET", baseURL+"/health", 200, "GET /health returned 200 (path ignored, not logged)")
	sleep()

	// ==================== Test 6: Panic recovery ====================
	fmt.Println("\n--- Test 6: Panic recovery ---\n")
	testRequest("GET", baseURL+"/panic", 500, "GET /panic returned 500 (middleware recovered from panic)")
	sleep()

	// ==================== Test 7: Slog capture ====================
	fmt.Println("\n--- Test 7: Slog capture ---\n")

	logdot.SetSlogCapture(logger)

	slog.Info("Slog capture test: info from Go hooks")
	slog.Warn("Slog capture test: warning from Go hooks")
	slog.Error("Slog capture test: error from Go hooks")

	pass("slog.Info/Warn/Error forwarded without error")
	sleep()

	// ==================== Test 8: Slog with attributes ====================
	fmt.Println("\n--- Test 8: Slog with structured attributes ---\n")

	slog.Info("Slog attrs test",
		"user_id", 42,
		"action", "login",
		"duration_ms", 123.45,
	)

	pass("slog with attrs forwarded without error")
	sleep()

	// ==================== Test 9: Slog with group ====================
	fmt.Println("\n--- Test 9: Slog with group ---\n")

	groupLogger := slog.Default().WithGroup("request")
	groupLogger.Info("Grouped slog test",
		"method", "GET",
		"path", "/api/users",
	)

	pass("slog with group forwarded without error")
	sleep()

	// Clean up
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	// ==================== Summary ====================
	printSummary()
}

func testRequest(method, url string, expectedStatus int, successMsg string) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fail(fmt.Sprintf("Failed to create request: %v", err))
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail(fmt.Sprintf("Request failed: %v", err))
		return
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == expectedStatus {
		pass(successMsg)
	} else {
		fail(fmt.Sprintf("Expected %d, got %d", expectedStatus, resp.StatusCode))
	}
}

func printSummary() {
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
		fmt.Println("\nAll tests passed! Go hooks are working correctly.")
		os.Exit(0)
	}
}
