package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

func setupTestStore(t *testing.T) *missions.MissionsStore {
	t.Helper()
	dir := t.TempDir()
	store := missions.NewMissionsStore(dir)
	return store
}

func createTestMission(t *testing.T, store *missions.MissionsStore, id, name string, status missions.MissionStatus) *missions.Mission {
	t.Helper()
	mission := &missions.Mission{
		ID:        id,
		Name:      name,
		Status:    status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Features: []missions.Feature{
			{ID: "feat-1", Description: "Feature 1", Status: missions.FeaturePending, Milestone: "M1"},
			{ID: "feat-2", Description: "Feature 2", Status: missions.FeatureCompleted, Milestone: "M1"},
			{ID: "feat-3", Description: "Feature 3", Status: missions.FeatureInProgress, Milestone: "M2"},
		},
		Milestones: []missions.Milestone{
			{Name: "M1", Description: "Milestone 1"},
			{Name: "M2", Description: "Milestone 2"},
		},
	}
	if err := store.CreateMission(mission); err != nil {
		t.Fatalf("failed to create test mission: %v", err)
	}
	return mission
}

func newTestServer(t *testing.T, store *missions.MissionsStore) *httptest.Server {
	t.Helper()
	s := New(0, store)
	return httptest.NewServer(s.Handler())
}

// mustGet issues a GET request and returns the response, failing the test on error.
func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// mustPost issues a POST request and returns the response, failing the test on error.
func mustPost(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "", nil)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// ─── Health Check ──────────────────────────────────────────────────────────

func TestHealthCheck(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/health")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
	if _, ok := body["version"]; !ok {
		t.Error("expected version field")
	}
	if _, ok := body["uptime"]; !ok {
		t.Error("expected uptime field")
	}
}

func TestHealthCheckContentType(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/health")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	cors := resp.Header.Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", cors)
	}
}

// ─── List Missions ─────────────────────────────────────────────────────────

func TestListMissionsEmpty(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var body struct {
		Missions []interface{} `json:"missions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Missions) != 0 {
		t.Errorf("expected empty list, got %d items", len(body.Missions))
	}
}

func TestListMissionsPopulated(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "mission-1", "Mission One", missions.MissionActive)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var body struct {
		Missions []map[string]interface{} `json:"missions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Missions) != 1 {
		t.Fatalf("expected 1 mission, got %d", len(body.Missions))
	}

	m := body.Missions[0]
	if m["id"] != "mission-1" {
		t.Errorf("expected id 'mission-1', got %q", m["id"])
	}
	if m["name"] != "Mission One" {
		t.Errorf("expected name 'Mission One', got %q", m["name"])
	}
	if m["status"] != "active" {
		t.Errorf("expected status 'active', got %q", m["status"])
	}
	if _, ok := m["featureCount"]; !ok {
		t.Error("expected featureCount field")
	}
	if _, ok := m["milestoneCount"]; !ok {
		t.Error("expected milestoneCount field")
	}
}

func TestListMissionsSortedByCreatedAt(t *testing.T) {
	store := setupTestStore(t)

	old := &missions.Mission{
		ID: "old", Name: "Old", Status: missions.MissionActive,
		CreatedAt: time.Now().Add(-1 * time.Hour), UpdatedAt: time.Now(),
	}
	newM := &missions.Mission{
		ID: "new", Name: "New", Status: missions.MissionActive,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := store.CreateMission(old); err != nil {
		t.Fatalf("failed to create old mission: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := store.CreateMission(newM); err != nil {
		t.Fatalf("failed to create new mission: %v", err)
	}

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions")
	defer resp.Body.Close()

	var body struct {
		Missions []map[string]interface{} `json:"missions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Missions) != 2 {
		t.Fatalf("expected 2 missions, got %d", len(body.Missions))
	}
	if body.Missions[0]["id"] != "new" {
		t.Errorf("expected newest first, got %q", body.Missions[0]["id"])
	}
}

func TestListMissionsFilterByStatus(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "active-1", "Active", missions.MissionActive)
	createTestMission(t, store, "paused-1", "Paused", missions.MissionPaused)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions?status=active")
	defer resp.Body.Close()

	var body struct {
		Missions []map[string]interface{} `json:"missions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode missions: %v", err)
	}

	if len(body.Missions) != 1 {
		t.Fatalf("expected 1 active mission, got %d", len(body.Missions))
	}
	if body.Missions[0]["id"] != "active-1" {
		t.Errorf("expected 'active-1', got %q", body.Missions[0]["id"])
	}
}

// ─── Get Mission ───────────────────────────────────────────────────────────

func TestGetMission(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "test-mission", "Test Mission", missions.MissionActive)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions/test-mission")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var mission missions.Mission
	if err := json.NewDecoder(resp.Body).Decode(&mission); err != nil {
		t.Fatalf("failed to decode mission: %v", err)
	}

	if mission.ID != "test-mission" {
		t.Errorf("expected id 'test-mission', got %q", mission.ID)
	}
	if mission.Name != "Test Mission" {
		t.Errorf("expected name 'Test Mission', got %q", mission.Name)
	}
	if mission.Status != missions.MissionActive {
		t.Errorf("expected status 'active', got %q", mission.Status)
	}
	if len(mission.Features) != 3 {
		t.Errorf("expected 3 features, got %d", len(mission.Features))
	}
	if len(mission.Milestones) != 2 {
		t.Errorf("expected 2 milestones, got %d", len(mission.Milestones))
	}
}

func TestGetMissionNotFound(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions/nonexistent")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected error message")
	}
}

// ─── List Features ─────────────────────────────────────────────────────────

func TestListFeatures(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "test-mission", "Test Mission", missions.MissionActive)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions/test-mission/features")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var features []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&features); err != nil {
		t.Fatalf("failed to decode features: %v", err)
	}
	if len(features) != 3 {
		t.Errorf("expected 3 features, got %d", len(features))
	}

	for i, f := range features {
		if _, ok := f["id"]; !ok {
			t.Errorf("feature[%d] missing id", i)
		}
		if _, ok := f["description"]; !ok {
			t.Errorf("feature[%d] missing description", i)
		}
		if _, ok := f["status"]; !ok {
			t.Errorf("feature[%d] missing status", i)
		}
		if _, ok := f["milestone"]; !ok {
			t.Errorf("feature[%d] missing milestone", i)
		}
	}
}

func TestListFeaturesNotFound(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions/nonexistent/features")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListFeaturesEmpty(t *testing.T) {
	store := setupTestStore(t)
	mission := &missions.Mission{
		ID: "empty", Name: "Empty", Status: missions.MissionActive,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Features: []missions.Feature{}, Milestones: []missions.Milestone{},
	}
	if err := store.CreateMission(mission); err != nil {
		t.Fatalf("failed to create empty mission: %v", err)
	}

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions/empty/features")
	defer resp.Body.Close()

	var features []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&features); err != nil {
		t.Fatalf("failed to decode features: %v", err)
	}

	if len(features) != 0 {
		t.Errorf("expected empty features list, got %d items", len(features))
	}
	if features == nil {
		t.Error("expected empty array [], not null")
	}
}

func TestListFeaturesFilterByStatus(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "test-mission", "Test Mission", missions.MissionActive)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions/test-mission/features?status=completed")
	defer resp.Body.Close()

	var features []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&features); err != nil {
		t.Fatalf("failed to decode features: %v", err)
	}

	if len(features) != 1 {
		t.Fatalf("expected 1 completed feature, got %d", len(features))
	}
	if features[0]["id"] != "feat-2" {
		t.Errorf("expected feat-2, got %q", features[0]["id"])
	}
}

// ─── Pause/Resume ──────────────────────────────────────────────────────────

func TestPauseMission(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "test-mission", "Test Mission", missions.MissionActive)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustPost(t, server.URL+"/api/missions/test-mission/pause")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var mission missions.Mission
	if err := json.NewDecoder(resp.Body).Decode(&mission); err != nil {
		t.Fatalf("failed to decode mission: %v", err)
	}
	if mission.Status != missions.MissionPaused {
		t.Errorf("expected status 'paused', got %q", mission.Status)
	}

	loaded, err := store.LoadMission("test-mission")
	if err != nil {
		t.Fatalf("failed to load mission: %v", err)
	}
	if loaded.Status != missions.MissionPaused {
		t.Errorf("expected persisted status 'paused', got %q", loaded.Status)
	}
}

func TestPauseInvalidState(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "test-mission", "Test Mission", missions.MissionCompleted)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustPost(t, server.URL+"/api/missions/test-mission/pause")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected error message")
	}
}

func TestResumeMission(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "test-mission", "Test Mission", missions.MissionPaused)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustPost(t, server.URL+"/api/missions/test-mission/resume")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var mission missions.Mission
	if err := json.NewDecoder(resp.Body).Decode(&mission); err != nil {
		t.Fatalf("failed to decode mission: %v", err)
	}
	if mission.Status != missions.MissionActive {
		t.Errorf("expected status 'active', got %q", mission.Status)
	}
}

func TestResumeInvalidState(t *testing.T) {
	store := setupTestStore(t)
	createTestMission(t, store, "test-mission", "Test Mission", missions.MissionActive)

	server := newTestServer(t, store)
	defer server.Close()

	resp := mustPost(t, server.URL+"/api/missions/test-mission/resume")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", resp.StatusCode)
	}
}

// ─── Error Response Schema ────────────────────────────────────────────────

func TestErrorResponseFormat(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/missions/nonexistent")
	defer resp.Body.Close()

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' field in 404 response")
	}
}

// ─── CORS Headers ──────────────────────────────────────────────────────────

func TestCORSHeaders(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/health")
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected Access-Control-Allow-Origin: *")
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type: application/json")
	}
}

// ─── Health Degraded ───────────────────────────────────────────────────────

func TestHealthCheckDegraded(t *testing.T) {
	store := missions.NewMissionsStore("/nonexistent/dir/that/does/not/exist")
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/health")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 200 or 503, got %d", resp.StatusCode)
	}
}

// ─── File Existence ────────────────────────────────────────────────────────

func TestUIFilesExist(t *testing.T) {
	entries, err := os.ReadDir("ui")
	if err != nil {
		t.Fatalf("failed to read ui directory: %v", err)
	}

	expected := map[string]bool{"index.html": false, "app.css": false, "app.js": false}
	for _, e := range entries {
		if !e.IsDir() {
			expected[e.Name()] = true
		}
	}

	for name, found := range expected {
		if !found {
			if _, err := os.Stat(filepath.Join("ui", name)); err == nil {
				continue
			}
			t.Errorf("expected UI file ui/%s not found", name)
		}
	}
}

// ─── Pause/Resume Not Found ────────────────────────────────────────────────

func TestPauseMissionNotFound(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustPost(t, server.URL+"/api/missions/nonexistent/pause")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestResumeMissionNotFound(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	resp := mustPost(t, server.URL+"/api/missions/nonexistent/resume")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// ─── Method Not Allowed ────────────────────────────────────────────────────

func TestMethodNotAllowed(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	// GET on a POST-only endpoint
	resp := mustGet(t, server.URL+"/api/missions/test-mission/pause")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 405 or 404, got %d", resp.StatusCode)
	}
}

// ─── Panic Recovery ────────────────────────────────────────────────────────

func TestPanicRecovery(t *testing.T) {
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	server := httptest.NewServer(handler)
	defer server.Close()

	resp := mustGet(t, server.URL+"/api/health")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// ─── Error schema for multiple endpoints ───────────────────────────────────

func TestErrorResponseSchemaAllEndpoints(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	endpoints := []string{
		"/api/missions/nonexistent",
		"/api/missions/nonexistent/features",
	}

	for _, ep := range endpoints {
		resp := mustGet(t, server.URL+ep)
		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			resp.Body.Close()
			t.Fatalf("failed to decode response for %s: %v", ep, err)
		}
		resp.Body.Close()

		if _, ok := body["error"]; !ok {
			t.Errorf("endpoint %s: expected 'error' field in response", ep)
		}
	}
}

// ─── VAL-WEB-010: Empty/null mission ID ────────────────────────────────────

func TestEmptyMissionIDReturns400(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	// GET /api/missions/ (empty ID)
	resp := mustGet(t, server.URL+"/api/missions/")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty mission ID, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' field in response")
	}
}

func TestNullByteMissionIDReturns400(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	// GET /api/missions/%00 (null byte)
	resp := mustGet(t, server.URL+"/api/missions/%00")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for null byte mission ID, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' field in response")
	}
}

// ─── VAL-WEB-047: 405 returns JSON ─────────────────────────────────────────

func TestMethodNotAllowedReturnsJSON(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	// POST on GET-only endpoint (health check)
	resp := mustPost(t, server.URL+"/api/health")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}

	// Verify JSON response
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json for 405, got %q", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body as JSON: %v", err)
	}

	errMsg, ok := body["error"]
	if !ok {
		t.Error("expected 'error' field in 405 response")
	} else if errMsg != "method not allowed" {
		t.Errorf("expected error message 'method not allowed', got %q", errMsg)
	}
}

func TestMethodNotAllowedReturnsJSONForAllEndpoints(t *testing.T) {
	store := setupTestStore(t)
	server := newTestServer(t, store)
	defer server.Close()

	// Test POST on GET-only endpoints
	postEndpoints := []string{
		"/api/health",
		"/api/missions/test-id",
	}

	for _, ep := range postEndpoints {
		resp := mustPost(t, server.URL+ep)
		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			resp.Body.Close()
			t.Fatalf("POST %s: failed to decode JSON response: %v", ep, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: expected 405, got %d", ep, resp.StatusCode)
		}
		if _, ok := body["error"]; !ok {
			t.Errorf("POST %s: expected 'error' field in 405 response", ep)
		}
	}
}
