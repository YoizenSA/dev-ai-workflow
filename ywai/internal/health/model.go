package health

import "time"

// HealthStatus represents the health state of ywai components.
type HealthStatus struct {
	DaemonOK  bool      `json:"daemon_ok"`
	DBOK      bool      `json:"db_ok"`
	RepoCount int       `json:"repo_count"`
	LastCheck time.Time `json:"last_check"`
}
