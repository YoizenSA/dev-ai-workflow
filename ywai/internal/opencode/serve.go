package opencode

// serve.go — shared "reuse a running opencode server" logic. Both the ywai
// CLI bootstrap (cmd/ywai startOpencodeServe) and the tools API
// (toolsapi StartOpencode) need to find an already-running instance before
// spawning a duplicate; this file is the single implementation.

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// DefaultPort is opencode's well-known serve port.
const DefaultPort = 4096

// StartPortFromURL extracts the port of an OPENCODE_URL-style URL, falling
// back to DefaultPort when the value is empty or has no parseable port.
func StartPortFromURL(u string) int {
	host := strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://")
	if _, p, err := net.SplitHostPort(host); err == nil && p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	return DefaultPort
}

// FindRunningURL probes 127.0.0.1 ports startPort..startPort+limit-1 for a
// healthy opencode server and returns the first URL that answers. extra,
// when non-nil, must also accept the candidate (e.g. an /app identity check
// so a non-opencode process squatting the port is rejected). Each probe gets
// a 500ms budget; ctx bounds the whole walk.
func FindRunningURL(ctx context.Context, startPort, limit int, extra func(ctx context.Context, url string) bool) (string, bool) {
	for p := startPort; p < startPort+limit; p++ {
		if ctx.Err() != nil {
			return "", false
		}
		candidate := fmt.Sprintf("http://127.0.0.1:%d", p)
		pctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		ok, _ := ProbeServer(pctx, candidate)
		cancel()
		if !ok {
			continue
		}
		if extra != nil {
			ectx, ecancel := context.WithTimeout(ctx, 500*time.Millisecond)
			ok = extra(ectx, candidate)
			ecancel()
		}
		if ok {
			return candidate, true
		}
	}
	return "", false
}
