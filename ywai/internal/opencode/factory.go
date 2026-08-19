package opencode

import (
	"context"
	"net/http"
	"os"
	"time"
)

const defaultProbeTimeout = 2 * time.Second

// ApplyServerAuth adds OpenCode's documented HTTP Basic authentication when
// the server password is provided to ywai's process environment.
func ApplyServerAuth(req *http.Request) {
	password := os.Getenv("OPENCODE_SERVER_PASSWORD")
	if password == "" {
		return
	}
	username := os.Getenv("OPENCODE_SERVER_USERNAME")
	if username == "" {
		username = "opencode"
	}
	req.SetBasicAuth(username, password)
}

type serverAuthTransport struct {
	base http.RoundTripper
}

func (t serverAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.Header = req.Header.Clone()
	ApplyServerAuth(copy)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(copy)
}

func authenticatedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: serverAuthTransport{}}
}

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

	ApplyServerAuth(req)
	client := authenticatedHTTPClient(defaultProbeTimeout)
	resp, err := client.Do(req)
	if err != nil {
		return false, nil //nolint:nilerr // not reachable, not an error
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK, nil
}
