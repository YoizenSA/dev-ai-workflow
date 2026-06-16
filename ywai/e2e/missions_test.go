package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions/web"
)

// ─── Helpers ───────────────────────────────────────────────────────────────

// missionsTestServer spins up a real missions HTTP server against a temp-backed
// store and returns the running server plus its base URL. The server is cleaned
// up automatically when the test ends.
func missionsTestServer(t *testing.T) (*httptest.Server, *missions.MissionsStore) {
	t.Helper()
	dir := t.TempDir()
	store := missions.NewMissionsStore(dir)
	srv := httptest.NewServer(web.New(0, store).Handler())
	t.Cleanup(srv.Close)
	return srv, store
}

// postJSON issues a POST with a JSON body and returns the status code + decoded
// body. It fails the test on transport errors but returns non-2xx as-is so the
// caller can assert on status codes.
func postJSON(t *testing.T, url string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]interface{}
	_ = json.Unmarshal(raw, &parsed)
	return resp.StatusCode, parsed
}

// getJSON issues a GET and returns the status code + decoded body.
func getJSON(t *testing.T, url string) (int, map[string]interface{}) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]interface{}
	_ = json.Unmarshal(raw, &parsed)
	return resp.StatusCode, parsed
}

// ─── mkdir endpoint (Bug A regression) ──────────────────────────────────────

// TestE2E_Mkdir_Create verifies POST /api/fs/mkdir creates a folder on disk and
// returns its absolute path. Regression guard for the empty-fields bug that
// caused session creation to fail with HTTP 400.
func TestE2E_Mkdir_Create(t *testing.T) {
	srv, _ := missionsTestServer(t)
	parent := t.TempDir()

	status, body := postJSON(t, srv.URL+"/api/fs/mkdir", map[string]string{
		"parentPath": parent,
		"name":       "my-mission",
	})
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %v)", status, body)
	}
	path, _ := body["path"].(string)
	if path == "" {
		t.Fatalf("expected non-empty path in response, got %v", body)
	}
	// The folder must exist on disk.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected folder %s to exist: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", path)
	}
	if filepath.Base(path) != "my-mission" {
		t.Errorf("folder name = %q, want my-mission", filepath.Base(path))
	}
}

// TestE2E_Mkdir_RejectsTraversal verifies the mkdir endpoint rejects names with
// path separators / traversal sequences, so it can't be used to write outside
// the chosen parent.
func TestE2E_Mkdir_RejectsTraversal(t *testing.T) {
	srv, _ := missionsTestServer(t)
	parent := t.TempDir()

	cases := []string{"a/b", "..", "evil/../escape", ""}
	for _, name := range cases {
		status, body := postJSON(t, srv.URL+"/api/fs/mkdir", map[string]string{
			"parentPath": parent,
			"name":       name,
		})
		if status != http.StatusBadRequest {
			t.Errorf("name %q: status = %d, want 400 (body: %v)", name, status, body)
		}
	}
}

// TestE2E_Mkdir_ConflictOnExisting verifies a duplicate folder name returns 409,
// not a silent success.
func TestE2E_Mkdir_ConflictOnExisting(t *testing.T) {
	srv, _ := missionsTestServer(t)
	parent := t.TempDir()

	// First create succeeds.
	if status, _ := postJSON(t, srv.URL+"/api/fs/mkdir", map[string]string{
		"parentPath": parent,
		"name":       "dup",
	}); status != http.StatusCreated {
		t.Fatalf("first create: status = %d, want 201", status)
	}
	// Second create must conflict.
	status, _ := postJSON(t, srv.URL+"/api/fs/mkdir", map[string]string{
		"parentPath": parent,
		"name":       "dup",
	})
	if status != http.StatusConflict {
		t.Errorf("duplicate create: status = %d, want 409", status)
	}
}

// ─── project registration (folder → project) ───────────────────────────────

// TestE2E_Project_CreateAndList verifies the flow the create-mission modal uses
// after picking a folder: register it as a project, then it shows up in the list.
func TestE2E_Project_CreateAndList(t *testing.T) {
	srv, _ := missionsTestServer(t)
	parent := t.TempDir()
	folder := filepath.Join(parent, "registered-proj")
	if err := os.Mkdir(folder, 0755); err != nil {
		t.Fatal(err)
	}

	// Register the folder as a project.
	status, body := postJSON(t, srv.URL+"/api/projects", map[string]string{
		"name": "registered-proj",
		"path": folder,
	})
	if status != http.StatusCreated {
		t.Fatalf("create project: status = %d, want 201 (body: %v)", status, body)
	}
	proj, ok := body["project"].(map[string]interface{})
	if !ok || proj["name"] != "registered-proj" {
		t.Fatalf("unexpected project in response: %v", body)
	}

	// It must appear in the list.
	status, body = getJSON(t, srv.URL+"/api/projects")
	if status != http.StatusOK {
		t.Fatalf("list projects: status = %d", status)
	}
	projects, _ := body["projects"].([]interface{})
	if len(projects) != 1 {
		t.Fatalf("expected 1 project in list, got %d (%v)", len(projects), body)
	}
}

// TestE2E_Project_DuplicateNameConflict verifies registering the same name twice
// returns 409 (the modal's handleSelectFolder relies on this to fall back to
// selecting the existing project).
func TestE2E_Project_DuplicateNameConflict(t *testing.T) {
	srv, _ := missionsTestServer(t)
	dir := t.TempDir()

	payload := map[string]string{"name": "dup-proj", "path": dir}
	if status, _ := postJSON(t, srv.URL+"/api/projects", payload); status != http.StatusCreated {
		t.Fatalf("first create: status = %d, want 201", status)
	}
	status, _ := postJSON(t, srv.URL+"/api/projects", payload)
	if status != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want 409", status)
	}
}

// ─── mission create via ApprovePlan (no opencode dependency) ───────────────

// TestE2E_Mission_ApproveAndList exercises the full mission lifecycle that does
// NOT depend on opencode: approve a hand-built plan, confirm the mission is
// persisted, then list and get it. This covers the code path the UI takes after
// "Generate Plan" + "Approve & Create".
func TestE2E_Mission_ApproveAndList(t *testing.T) {
	srv, _ := missionsTestServer(t)

	plan := map[string]interface{}{
		"name":        "E2E Test Mission",
		"description": "Mission created from an approved plan",
		"milestones": []map[string]string{
			{"name": "M1", "description": "First milestone"},
		},
		"features": []map[string]interface{}{
			{
				"id":          "feat-1",
				"description": "Implement feature one",
				"skillName":   "backend-worker",
				"milestone":   "M1",
			},
		},
	}

	// Approve the plan → creates + persists the mission.
	status, body := postJSON(t, srv.URL+"/api/missions/approve", map[string]interface{}{
		"plan": plan,
	})
	if status != http.StatusCreated {
		t.Fatalf("approve: status = %d, want 201 (body: %v)", status, body)
	}
	mission, ok := body["mission"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mission object in response, got %v", body)
	}
	missionID, _ := mission["id"].(string)
	if missionID == "" {
		t.Fatalf("mission has no id: %v", mission)
	}
	if mission["name"] != "E2E Test Mission" {
		t.Errorf("mission name = %v, want E2E Test Mission", mission["name"])
	}

	// List missions — the new one must appear.
	status, body = getJSON(t, srv.URL+"/api/missions")
	if status != http.StatusOK {
		t.Fatalf("list: status = %d", status)
	}
	list, _ := body["missions"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 mission in list, got %d", len(list))
	}

	// Get the specific mission.
	status, body = getJSON(t, srv.URL+"/api/missions/"+missionID)
	if status != http.StatusOK {
		t.Fatalf("get: status = %d", status)
	}
	if body["id"] != missionID {
		t.Errorf("get returned id %v, want %s", body["id"], missionID)
	}
}

// ─── opencode status / start (Bug B regression) ────────────────────────────

// TestE2E_OpenCode_StatusReachable verifies the status endpoint responds with a
// JSON shape the frontend can consume (connected + source fields). It accepts
// any source value — in CI the opencode binary may not be installed.
func TestE2E_OpenCode_StatusReachable(t *testing.T) {
	srv, _ := missionsTestServer(t)

	status, body := getJSON(t, srv.URL+"/api/opencode/status")
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body: %v)", status, body)
	}
	// Must contain the fields the frontend reads.
	if _, ok := body["connected"]; !ok {
		t.Errorf("status response missing 'connected' field: %v", body)
	}
}

// TestE2E_OpenCode_StartIsIdempotent verifies that calling /opencode/start does
// not crash the endpoint. Whether it actually launches opencode depends on the
// host, so we accept 500 only when the binary is not installed (CI) — any other
// 500 is a real error.
func TestE2E_OpenCode_StartIsIdempotent(t *testing.T) {
	srv, _ := missionsTestServer(t)

	for i := 0; i < 2; i++ {
		status, body := postJSON(t, srv.URL+"/api/opencode/start", map[string]string{})
		if status >= 500 {
			errMsg, _ := body["error"].(string)
			if errMsg == "" {
				errMsg, _ = body["message"].(string)
			}
			// CI runners don't have opencode installed — accept the 500.
			if strings.Contains(errMsg, "executable file not found") {
				continue
			}
			t.Errorf("call %d: status = %d, want < 500 (body: %v)", i, status, body)
		}
	}
}

// ─── health ─────────────────────────────────────────────────────────────────

// TestE2E_Health verifies the health endpoint returns ok status — this is what
// the dashboard polls to know the server is alive.
func TestE2E_Health(t *testing.T) {
	srv, _ := missionsTestServer(t)
	status, body := getJSON(t, srv.URL+"/api/health")
	if status != http.StatusOK {
		t.Fatalf("health: status = %d, want 200", status)
	}
	if body["status"] != "ok" {
		t.Errorf("health status = %v, want 'ok'", body["status"])
	}
}

// ─── error shape consistency ────────────────────────────────────────────────

// TestE2E_ErrorShapeIsJSON verifies that error responses use the {error: "..."}
// shape the frontend expects, not Go's default plaintext. This guards against
// regressions in writeError.
func TestE2E_ErrorShapeIsJSON(t *testing.T) {
	srv, _ := missionsTestServer(t)

	// Approving a missing plan → 400 with {error: ...}.
	status, body := postJSON(t, srv.URL+"/api/missions/approve", map[string]interface{}{})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	errMsg, _ := body["error"].(string)
	if errMsg == "" {
		// Some errors return a bare string; check raw too.
		if raw, _ := body["error"]; raw == nil {
			t.Errorf("expected non-empty 'error' field, got %v", body)
		}
	}
	if !strings.Contains(errMsg, "plan") && errMsg != "" {
		t.Logf("note: error message %q does not mention 'plan'", errMsg)
	}
}
