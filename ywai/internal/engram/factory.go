package engram

import (
	"context"
	"net/http"
	"os"
	"time"
)

const defaultProbeTimeout = 2 * time.Second

// DefaultClient returns an HTTPClient pointing at the engram server
// (http://127.0.0.1:7437 by default; override with ENGRAM_URL).
// It does NOT probe — callers should use Status() to check connectivity.
func DefaultClient() *HTTPClient {
	url := os.Getenv("ENGRAM_URL")
	if url == "" {
		url = "http://127.0.0.1:7437"
	}
	return NewHTTPClient(url)
}

// ProbeServer reports whether the engram server is reachable at baseURL.
func ProbeServer(ctx context.Context, baseURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
