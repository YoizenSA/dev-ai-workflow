package engram

import (
	"os"
)

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
