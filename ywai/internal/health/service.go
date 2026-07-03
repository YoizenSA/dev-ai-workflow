package health

import (
	"context"
	"net"
	"os"
	"time"
)

// Service checks health of ywai components.
type Service struct {
	dbPath     string
	daemonAddr string
}

// NewService creates a health service for the given db path and daemon address.
func NewService(dbPath, daemonAddr string) *Service {
	return &Service{dbPath: dbPath, daemonAddr: daemonAddr}
}

// CheckHealth checks all components and returns a HealthStatus.
func (s *Service) CheckHealth(ctx context.Context) (*HealthStatus, error) {
	return &HealthStatus{
		DaemonOK:  s.checkDaemon(),
		DBOK:      s.checkDB(),
		RepoCount: 0,
		LastCheck: time.Now(),
	}, nil
}

func (s *Service) checkDaemon() bool {
	if s.daemonAddr == "" {
		return true
	}
	conn, err := net.DialTimeout("tcp", s.daemonAddr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (s *Service) checkDB() bool {
	if s.dbPath == ":memory:" {
		return true
	}
	_, err := os.Stat(s.dbPath)
	return err == nil
}
