package scheduler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// setupTestHandler creates a scheduler with a fake store and returns its HTTP handler.
func setupTestHandler(t *testing.T) *Handler {
	t.Helper()
	store := newFakeStore()
	sch := NewScheduler(store)
	return NewHandler(sch)
}

func apiURL(path string) string {
	return path
}

func TestAPI_GetSchedules_Empty(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + apiURL("/api/schedules"))
	if err != nil {
		t.Fatalf("GET /api/schedules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var schedules []Schedule
	if err := json.NewDecoder(resp.Body).Decode(&schedules); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("expected empty list, got %d items", len(schedules))
	}
}

func TestAPI_GetSchedules_AfterCreate(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	// Create a schedule first
	body := `{"config":{"goal":"API test","repo":"org/api","agent":"dev"},"cronExpr":"@every 1h","enabled":true}`
	resp, err := http.Post(server.URL+apiURL("/api/schedules"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/schedules failed: %v", err)
	}
	resp.Body.Close()

	// Now list
	resp, err = http.Get(server.URL + apiURL("/api/schedules"))
	if err != nil {
		t.Fatalf("GET /api/schedules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var schedules []Schedule
	if err := json.NewDecoder(resp.Body).Decode(&schedules); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].Config.Goal != "API test" {
		t.Errorf("Goal = %q, want %q", schedules[0].Config.Goal, "API test")
	}
}

func TestAPI_CreateSchedule_201(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	body := `{"config":{"goal":"Create test","repo":"org/create","agent":"dev"},"cronExpr":"@daily","enabled":true}`
	resp, err := http.Post(server.URL+apiURL("/api/schedules"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/schedules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var created Schedule
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if created.ID == "" {
		t.Error("created schedule should have non-empty ID")
	}
	if created.CronExpr != "@daily" {
		t.Errorf("CronExpr = %q, want %q", created.CronExpr, "@daily")
	}
}

func TestAPI_CreateSchedule_InvalidJSON_400(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Post(server.URL+apiURL("/api/schedules"), "application/json", strings.NewReader(`{invalid json`))
	if err != nil {
		t.Fatalf("POST /api/schedules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAPI_CreateSchedule_MissingFields_400(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	body := `{"cronExpr":"@every 1h"}` // missing config
	resp, err := http.Post(server.URL+apiURL("/api/schedules"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/schedules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAPI_CreateSchedule_InvalidCron_400(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	body := `{"config":{"goal":"Bad cron","repo":"org/bad","agent":"dev"},"cronExpr":"invalid!!!","enabled":true}`
	resp, err := http.Post(server.URL+apiURL("/api/schedules"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/schedules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAPI_DeleteSchedule_204(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	// Create a schedule first
	body := `{"config":{"goal":"Delete me","repo":"org/del","agent":"qa"},"cronExpr":"@every 1h","enabled":true}`
	resp, err := http.Post(server.URL+apiURL("/api/schedules"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	var created Schedule
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// Delete it
	req, _ := http.NewRequest(http.MethodDelete, server.URL+apiURL("/api/schedules/"+created.ID), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestAPI_DeleteSchedule_NotFound_404(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodDelete, server.URL+apiURL("/api/schedules/nonexistent"), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAPI_GetRuns_Empty(t *testing.T) {
	h := setupTestHandler(t)
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + apiURL("/api/schedules/sched-1/runs"))
	if err != nil {
		t.Fatalf("GET /api/schedules/sched-1/runs failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var runs []ScheduleRun
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected empty runs list, got %d", len(runs))
	}
}

func TestAPI_GetRuns_AfterTick(t *testing.T) {
	store := newFakeStore()
	sch := NewScheduler(store)
	h := NewHandler(sch)
	server := httptest.NewServer(h)
	defer server.Close()

	// Create a schedule and make it due
	past := timeNow().Add(-5 * time.Minute)
	body := `{"config":{"goal":"Runs test","repo":"org/runs","agent":"dev"},"cronExpr":"@every 1h","enabled":true}`
	resp, err := http.Post(server.URL+apiURL("/api/schedules"), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	var created Schedule
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// Set NextRun to the past
	stored, _ := store.GetSchedule(created.ID)
	stored.NextRun = past
	store.UpdateSchedule(stored)

	// Tick
	_, _ = sch.Tick()

	// Get runs
	resp, err = http.Get(server.URL + apiURL("/api/schedules/"+created.ID+"/runs"))
	if err != nil {
		t.Fatalf("GET runs failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var runs []ScheduleRun
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].ScheduleID != created.ID {
		t.Errorf("run ScheduleID = %q, want %q", runs[0].ScheduleID, created.ID)
	}
}
