package opencode

import (
	"context"
	"net/http"
	"os"
	"time"
)

const defaultProbeTimeout = 2 * time.Second

// DefaultClient tries the server first; falls back to local config.
// Set OPENCODE_URL env var to override the default server URL.
func DefaultClient(ctx context.Context) Client {
	url := os.Getenv("OPENCODE_URL")
	if url == "" {
		url = "http://127.0.0.1:4096"
	}
	serverClient := NewServerClient(url)
	status, err := serverClient.Status(ctx)
	if err == nil && status.Connected {
		return serverClient
	}
	return NewLocalClient()
}

// ProbeServer checks if the opencode server is reachable at the given URL.
func ProbeServer(ctx context.Context, baseURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/status", nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil //nolint:nilerr // not reachable, not an error
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}
