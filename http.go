package logdot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

const (
	baseLogsURL    = "https://logs.logdot.io/api/v1"
	baseMetricsURL = "https://metrics.logdot.io/api/v1"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// HTTPClient handles HTTP communication with retry logic
type HTTPClient struct {
	client    *http.Client
	apiKey    string
	timeout   time.Duration
	retry     RetryConfig
	debug     bool
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient(apiKey string, timeout time.Duration, retry RetryConfig, debug bool) *HTTPClient {
	return &HTTPClient{
		client:  &http.Client{Timeout: timeout},
		apiKey:  apiKey,
		timeout: timeout,
		retry:   retry,
		debug:   debug,
	}
}

// Post performs a POST request with retry
func (h *HTTPClient) Post(ctx context.Context, url string, body interface{}) (*http.Response, []byte, error) {
	return h.doWithRetry(ctx, "POST", url, body)
}

// Get performs a GET request with retry
func (h *HTTPClient) Get(ctx context.Context, url string) (*http.Response, []byte, error) {
	return h.doWithRetry(ctx, "GET", url, nil)
}

func (h *HTTPClient) doWithRetry(ctx context.Context, method, url string, body interface{}) (*http.Response, []byte, error) {
	var lastErr error

	for attempt := 0; attempt < h.retry.MaxAttempts; attempt++ {
		resp, respBody, err := h.doRequest(ctx, method, url, body)
		if err == nil {
			return resp, respBody, nil
		}

		lastErr = err

		if attempt < h.retry.MaxAttempts-1 {
			delay := h.calculateBackoff(attempt)
			h.log("Retry %d/%d after %v - Error: %v", attempt+1, h.retry.MaxAttempts, delay, err)

			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return nil, nil, lastErr
}

func (h *HTTPClient) doRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, []byte, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
		h.log("%s %s", method, url)
		h.log("Payload: %s", string(jsonBody))
	} else {
		h.log("%s %s", method, url)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}

	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	h.log("Response status: %d", resp.StatusCode)
	if len(respBody) > 0 {
		h.log("Response body: %s", string(respBody))
	}

	return resp, respBody, nil
}

func (h *HTTPClient) calculateBackoff(attempt int) time.Duration {
	delay := float64(h.retry.BaseDelay) * math.Pow(2, float64(attempt))
	jitter := rand.Float64() * 0.3 * delay
	total := time.Duration(delay + jitter)

	if total > h.retry.MaxDelay {
		return h.retry.MaxDelay
	}
	return total
}

func (h *HTTPClient) log(format string, args ...interface{}) {
	if h.debug {
		fmt.Printf("[LogDot] "+format+"\n", args...)
	}
}
