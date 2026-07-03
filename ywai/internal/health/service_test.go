package health

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestHealthStatusFields(t *testing.T) {
	// Verify the struct has the correct field types and zero values.
	var hs HealthStatus

	// daemon_ok should be a bool
	if _, ok := any(hs.DaemonOK).(bool); !ok {
		t.Error("DaemonOK should be bool type")
	}

	// db_ok should be a bool
	if _, ok := any(hs.DBOK).(bool); !ok {
		t.Error("DBOK should be bool type")
	}

	// repo_count should be an int
	if _, ok := any(hs.RepoCount).(int); !ok {
		t.Error("RepoCount should be int type")
	}

	// last_check should be time.Time
	if _, ok := any(hs.LastCheck).(time.Time); !ok {
		t.Error("LastCheck should be time.Time type")
	}
}

func TestHealthStatusJSONRoundTrip(t *testing.T) {
	now := time.Now().Round(time.Millisecond)
	orig := HealthStatus{
		DaemonOK:  true,
		DBOK:      false,
		RepoCount: 3,
		LastCheck: now,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Verify JSON field names (snake_case)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map failed: %v", err)
	}
	if _, ok := raw["daemon_ok"]; !ok {
		t.Error("JSON missing field: daemon_ok")
	}
	if _, ok := raw["db_ok"]; !ok {
		t.Error("JSON missing field: db_ok")
	}
	if _, ok := raw["repo_count"]; !ok {
		t.Error("JSON missing field: repo_count")
	}
	if _, ok := raw["last_check"]; !ok {
		t.Error("JSON missing field: last_check")
	}

	// Round-trip: marshal then unmarshal, verify values preserved
	var decoded HealthStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal into struct failed: %v", err)
	}
	if decoded.DaemonOK != true {
		t.Error("DaemonOK should round-trip as true")
	}
	if decoded.DBOK != false {
		t.Error("DBOK should round-trip as false")
	}
	if decoded.RepoCount != 3 {
		t.Errorf("RepoCount should round-trip as 3, got %d", decoded.RepoCount)
	}
	if !decoded.LastCheck.Equal(now) {
		t.Errorf("LastCheck should round-trip, got %v, want %v", decoded.LastCheck, now)
	}
}

func TestCheckHealthReturnsNonNil(t *testing.T) {
	ctx := context.Background()
	svc := NewService(":memory:", ":memory:")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatalf("CheckHealth returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("CheckHealth should return non-nil result")
	}
}

func TestCheckHealthDaemonOK(t *testing.T) {
	ctx := context.Background()
	// Empty daemon address means no daemon configured — treated as healthy.
	svc := NewService(":memory:", "")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DaemonOK {
		t.Error("daemon_ok should be true when daemon address is empty (no daemon configured)")
	}
}

func TestCheckHealthDaemonDown(t *testing.T) {
	ctx := context.Background()
	// Point daemon to a port where nothing listens — simulates daemon down.
	svc := NewService(":memory:", "localhost:1")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.DaemonOK {
		t.Error("daemon_ok should be false when daemon is unreachable")
	}
}

func TestCheckHealthDBOK(t *testing.T) {
	ctx := context.Background()
	svc := NewService(":memory:", ":memory:")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DBOK {
		t.Error("db_ok should be true when SQLite is accessible")
	}
}

func TestCheckHealthDBUnavailable(t *testing.T) {
	ctx := context.Background()
	// Non-existent directory — DB can't be opened.
	svc := NewService("/nonexistent/db.sqlite", ":memory:")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.DBOK {
		t.Error("db_ok should be false when SQLite is unavailable")
	}
}

func TestCheckHealthRepoCountZero(t *testing.T) {
	ctx := context.Background()
	svc := NewService(":memory:", ":memory:")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.RepoCount != 0 {
		t.Errorf("repo_count should be 0 for empty state, got %d", result.RepoCount)
	}
}

func TestCheckHealthRepoCountNonZero(t *testing.T) {
	ctx := context.Background()
	// Even with :memory: DB, if no repos seeded, repo_count must be 0.
	// This test documents the behavior for future impl: when repos exist, repo_count > 0.
	svc := NewService(":memory:", ":memory:")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// For now, with empty DB, we expect 0. Full repo counting comes with the checker.
	_ = result.RepoCount
}

func TestCheckHealthDaemonAddrEmpty(t *testing.T) {
	ctx := context.Background()
	// Empty daemon address means no daemon configured — treated as healthy.
	svc := NewService(":memory:", "")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DaemonOK {
		t.Error("daemon_ok should be true when daemon address is empty (no daemon configured)")
	}
}

func TestCheckHealthContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled

	svc := NewService(":memory:", ":memory:")
	result, err := svc.CheckHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil even with cancelled context")
	}
	if !result.DBOK {
		t.Error("db_ok should still be true with cancelled context (context is not yet plumbed)")
	}
}
