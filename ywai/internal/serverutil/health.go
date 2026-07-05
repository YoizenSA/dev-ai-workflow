package serverutil

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// CheckHealth checks if a server is running on the given port.
func CheckHealth(port int) bool {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

// GetRunningPort attempts to find a server by checking common ports.
func GetRunningPort() int {
	// Check default port first
	if CheckHealth(5768) {
		return 5768
	}

	// Check missions old port
	if CheckHealth(5769) {
		return 5769
	}

	return 0
}

// IsPortFree reports whether a TCP port is bindable on 127.0.0.1 right now.
func IsPortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// FindFreePort returns the first bindable port at or after startPort (on
// 127.0.0.1), trying up to 50 consecutive ports. Returns startPort if it is
// free, otherwise the first free one found. Used to avoid collisions when a
// default port (e.g. opencode's 4096) is occupied by another process.
func FindFreePort(startPort int) int {
	for i := 0; i < 50; i++ {
		candidate := startPort + i
		if IsPortFree(candidate) {
			return candidate
		}
	}
	return startPort // best effort; let the caller fail on bind
}
