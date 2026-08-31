package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const defaultProbeTimeout = 2 * time.Second

// serverAuthFile is where `ywai serve` persists the Basic Auth credentials it
// generated for its opencode2 child. Child processes of the daemon (chat proxy,
// evals runner, mission workers) may not inherit OPENCODE_SERVER_PASSWORD from
// the launching shell, so ApplyServerAuth falls back to this file to reach the
// server ywai actually started.
const serverAuthFile = ".ywai/opencode-server-auth.json"

type serverAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loadServerAuth returns credentials from the environment first, then from the
// persisted ywai server-auth file.
func loadServerAuth() serverAuth {
	if u, p := os.Getenv("OPENCODE_SERVER_USERNAME"), os.Getenv("OPENCODE_SERVER_PASSWORD"); p != "" {
		username := u
		if username == "" {
			username = "opencode"
		}
		return serverAuth{Username: username, Password: p}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return serverAuth{}
	}
	raw, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(serverAuthFile)))
	if err != nil {
		return serverAuth{}
	}
	var a serverAuth
	if err := json.Unmarshal(raw, &a); err != nil || a.Password == "" {
		return serverAuth{}
	}
	return a
}

// SaveServerAuth persists the credentials that `ywai serve` generated for its
// opencode2 child so later processes in the same session can authenticate.
func SaveServerAuth(username, password string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, filepath.FromSlash(serverAuthFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, _ := json.Marshal(serverAuth{Username: username, Password: password})
	return os.WriteFile(path, raw, 0o600)
}

// ApplyServerAuth adds OpenCode's documented HTTP Basic authentication when
// the server credentials are available (env or the persisted ywai auth file).
func ApplyServerAuth(req *http.Request) {
	a := loadServerAuth()
	if a.Password == "" {
		return
	}
	req.SetBasicAuth(a.Username, a.Password)
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
