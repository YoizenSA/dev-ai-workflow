package serverutil

import (
	"fmt"
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
