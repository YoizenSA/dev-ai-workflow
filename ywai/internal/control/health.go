package control

import (
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/serverutil"
)

// CheckHealth checks if the control server is running on the given port.
func CheckHealth(port int) bool {
	return serverutil.CheckHealth(port)
}

// GetRunningPort attempts to find the control server by checking common ports.
func GetRunningPort() int {
	return serverutil.GetRunningPort()
}
