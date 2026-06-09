package opencode

import (
	"context"
	"net/http"
	"time"
)

const defaultProbeTimeout = 2 * time.Second

// DefaultClient tries the server first; falls back to local config.
func DefaultClient(ctx context.Context) Client {
	serverClient := NewServerClient("http://127.0.0.1:4096")
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
