package control

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The tools API mux registers its own patterns; the control server has to
// forward each one. A path it does not forward is not a 404 — the SPA
// catch-all answers 200 with index.html, so an API call sees HTML and a
// websocket handshake fails with "Unexpected response code: 200".
func TestEveryToolsAPIRouteIsProxied(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "toolsapi", "server.go"))
	if err != nil {
		t.Fatalf("read toolsapi server: %v", err)
	}
	// mux.HandleFunc("GET /engram/ws", ...) → /engram/ws
	re := regexp.MustCompile(`mux\.HandleFunc\("(?:[A-Z]+ )?(/[^"]*)"`)

	proxied := make(map[string]bool, len(toolsAPIProxyPaths))
	for _, p := range toolsAPIProxyPaths {
		proxied[strings.TrimPrefix(p, "/missions")] = true
	}

	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		path := m[1]
		if proxied[path] {
			continue
		}
		// Prefix patterns cover everything beneath them.
		covered := false
		for p := range proxied {
			if strings.HasSuffix(p, "/") && strings.HasPrefix(path, p) {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("toolsapi serves %q but the control server does not proxy it; "+
				"requests fall through to the SPA catch-all and get 200 + index.html", path)
		}
	}
}
