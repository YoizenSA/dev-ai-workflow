package kanban

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestServer creates a kanban server on a random port, starts it,
// and returns the server and base URL. The caller must defer s.Stop().
func setupTestServer(t *testing.T) (*Server, string) {
	dir := t.TempDir()
	s := New(0, dir)
	go func() {
		if err := s.Start(); err != nil {
			// Server may fail to start (e.g., port conflict); log and skip.
			t.Logf("server Start returned: %v", err)
		}
	}()

	// Wait for the server to be ready and get the actual port
	var baseURL string
	client := &http.Client{Timeout: 1 * time.Second}

	for i := 0; i < 100; i++ {
		port := s.Port()
		if port == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		baseURL = fmt.Sprintf("http://localhost:%d", port)
		resp, err := client.Get(baseURL + "/api/sessions")
		if err == nil {
			resp.Body.Close()
			return s, baseURL
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server did not start within timeout, last port: %d", s.Port())
	return s, "" // unreachable
}

// setupTestData creates a session and a delegation for activity tests.
func setupTestData(t *testing.T, baseURL string) (sessionID, delegationID string) {
	// Create session
	sessionPayload := map[string]string{
		"project": "test-project",
		"goal":    "Activity handler tests",
	}
	body, _ := json.Marshal(sessionPayload)
	resp, err := http.Post(baseURL+"/api/sessions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	defer resp.Body.Close()

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode session failed: %v", err)
	}

	// Create delegation
	delPayload := map[string]interface{}{
		"session_id":   session.ID,
		"agent":        "dev",
		"task_summary": "Test task",
		"dependencies": []string{},
	}
	body, _ = json.Marshal(delPayload)
	resp, err = http.Post(baseURL+"/api/delegations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create delegation failed: %v", err)
	}
	defer resp.Body.Close()

	var del Delegation
	if err := json.NewDecoder(resp.Body).Decode(&del); err != nil {
		t.Fatalf("decode delegation failed: %v", err)
	}

	return session.ID, del.ID
}

func TestHandlers_CreateActivity(t *testing.T) {
	s, baseURL := setupTestServer(t)
	defer s.Stop()

	_, delID := setupTestData(t, baseURL)

	// Test POST 201 — valid activity
	payload := map[string]interface{}{
		"type":    "progress",
		"content": "Starting implementation",
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/delegations/%s/activities", baseURL, delID)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST activity failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var activity ActivityEvent
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		t.Fatalf("decode activity failed: %v", err)
	}
	if activity.ID == "" {
		t.Error("activity ID should be set")
	}
	if activity.Type != ActivityProgress {
		t.Errorf("expected type 'progress', got '%s'", activity.Type)
	}

	// Test POST 404 — invalid delegation ID
	resp, err = http.Post(
		fmt.Sprintf("%s/api/delegations/nonexistent/activities", baseURL),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST invalid delegation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for invalid delegation, got %d", resp.StatusCode)
	}

	// Test POST 400 — bad JSON
	resp, err = http.Post(url, "application/json", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("POST bad JSON failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", resp.StatusCode)
	}
}

func TestHandlers_GetActivities(t *testing.T) {
	s, baseURL := setupTestServer(t)
	defer s.Stop()

	_, delID := setupTestData(t, baseURL)

	// Test GET 200 — empty array initially
	url := fmt.Sprintf("%s/api/delegations/%s/activities", baseURL, delID)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET activities failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var activities []ActivityEvent
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		t.Fatalf("decode activities failed: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("expected empty array, got %d activities", len(activities))
	}

	// Add an activity via API
	payload := map[string]interface{}{
		"type":    "decision",
		"content": "Approve design?",
		"options": []string{"approve", "reject"},
	}
	body, _ := json.Marshal(payload)
	postResp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST activity failed: %v", err)
	}
	_ = postResp.Body.Close()

	// GET again — should have one activity
	resp, err = http.Get(url)
	if err != nil {
		t.Fatalf("GET activities failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	activities = nil
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		t.Fatalf("decode activities failed: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].Type != ActivityDecision {
		t.Errorf("expected decision type, got '%s'", activities[0].Type)
	}

	// Test GET — non-existent delegation returns 200 with empty array
	// (store returns empty slice with no error for unknown delegation IDs)
	resp, err = http.Get(fmt.Sprintf("%s/api/delegations/nonexistent/activities", baseURL))
	if err != nil {
		t.Fatalf("GET invalid delegation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent delegation, got %d", resp.StatusCode)
	}
}

func TestHandlers_ResolveActivity(t *testing.T) {
	s, baseURL := setupTestServer(t)
	defer s.Stop()

	_, delID := setupTestData(t, baseURL)

	// Create a decision activity via API
	payload := map[string]interface{}{
		"type":    "decision",
		"content": "Approve design?",
		"options": []string{"approve", "reject"},
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/delegations/%s/activities", baseURL, delID)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST activity failed: %v", err)
	}
	var activity ActivityEvent
	_ = json.NewDecoder(resp.Body).Decode(&activity)
	resp.Body.Close()

	// Test PATCH 200 — resolve the activity
	resolvePayload := map[string]string{"resolution": "approve"}
	resolveBody, _ := json.Marshal(resolvePayload)
	resolveURL := fmt.Sprintf("%s/api/delegations/%s/activities/%s", baseURL, delID, activity.ID)
	req, err := http.NewRequest(http.MethodPatch, resolveURL, bytes.NewReader(resolveBody))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH resolve failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200 for resolve, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var resolved ActivityEvent
	_ = json.NewDecoder(resp.Body).Decode(&resolved)
	if resolved.Resolution != "approve" {
		t.Errorf("expected resolution 'approve', got '%s'", resolved.Resolution)
	}
	if resolved.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}

	// Test PATCH 404 — invalid delegation ID
	req, _ = http.NewRequest(http.MethodPatch,
		fmt.Sprintf("%s/api/delegations/nonexistent/activities/%s", baseURL, activity.ID),
		bytes.NewReader(resolveBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH invalid delegation failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for invalid delegation, got %d", resp.StatusCode)
	}

	// Test PATCH 404 — invalid activity ID
	req, _ = http.NewRequest(http.MethodPatch,
		fmt.Sprintf("%s/api/delegations/%s/activities/nonexistent", baseURL, delID),
		bytes.NewReader(resolveBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH invalid activity failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for invalid activity, got %d", resp.StatusCode)
	}
}

func TestHandlers_GetPendingDecisions(t *testing.T) {
	s, baseURL := setupTestServer(t)
	defer s.Stop()

	sessionID, delID := setupTestData(t, baseURL)

	// Test GET 200 — no pending decisions initially
	resp, err := http.Get(fmt.Sprintf("%s/api/sessions/%s/decisions", baseURL, sessionID))
	if err != nil {
		t.Fatalf("GET pending decisions failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var pending []ActivityEvent
	if err := json.NewDecoder(resp.Body).Decode(&pending); err != nil {
		t.Fatalf("decode pending failed: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected empty pending, got %d", len(pending))
	}

	// Add a decision activity
	payload := map[string]interface{}{
		"type":    "decision",
		"content": "Go/no-go?",
		"options": []string{"go", "no-go"},
	}
	body, _ := json.Marshal(payload)
	postResp, _ := http.Post(
		fmt.Sprintf("%s/api/delegations/%s/activities", baseURL, delID),
		"application/json",
		bytes.NewReader(body),
	)
	_ = postResp.Body.Close()

	// Add a progress activity (should NOT appear in pending)
	payload = map[string]interface{}{
		"type":    "progress",
		"content": "Working on it",
	}
	body, _ = json.Marshal(payload)
	postResp, _ = http.Post(
		fmt.Sprintf("%s/api/delegations/%s/activities", baseURL, delID),
		"application/json",
		bytes.NewReader(body),
	)
	_ = postResp.Body.Close()

	// Test GET 200 — filtered results (only decision, not progress)
	resp, err = http.Get(fmt.Sprintf("%s/api/sessions/%s/decisions", baseURL, sessionID))
	if err != nil {
		t.Fatalf("GET pending decisions failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	pending = nil
	if err := json.NewDecoder(resp.Body).Decode(&pending); err != nil {
		t.Fatalf("decode pending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending decision, got %d", len(pending))
	}
	if pending[0].Type != ActivityDecision {
		t.Errorf("expected decision type, got '%s'", pending[0].Type)
	}
	// Progress should NOT be in the pending list
	foundProgress := false
	for _, p := range pending {
		if p.Type == ActivityProgress {
			foundProgress = true
		}
	}
	if foundProgress {
		t.Error("progress activity should not appear in pending decisions")
	}

	// Test GET 200 — invalid session returns empty array (not 404 since lookups may just return empty)
	resp, err = http.Get(fmt.Sprintf("%s/api/sessions/nonexistent/decisions", baseURL))
	if err != nil {
		t.Fatalf("GET pending decisions for non-existent session failed: %v", err)
	}
	defer resp.Body.Close()

	// The GetPendingDecisions handler returns 404 if store returns an error,
	// but the store returns an empty array (no error) for non-existent sessions.
	// Check that we get a non-error HTTP response.
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 2xx, got %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

func TestHandlers_CreateActivity_Validation(t *testing.T) {
	s, baseURL := setupTestServer(t)
	defer s.Stop()

	_, delID := setupTestData(t, baseURL)

	url := fmt.Sprintf("%s/api/delegations/%s/activities", baseURL, delID)

	// Test POST 400 — missing type
	payload := map[string]interface{}{
		"content": "Missing type field",
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST missing type failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing type, got %d", resp.StatusCode)
	}

	// Test POST 400 — missing content
	payload = map[string]interface{}{
		"type": "progress",
	}
	body, _ = json.Marshal(payload)
	resp, err = http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST missing content failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing content, got %d", resp.StatusCode)
	}

	// Test POST 201 — with options
	payload = map[string]interface{}{
		"type":    "question",
		"content": "Which approach?",
		"options": []string{"A", "B", "C"},
	}
	body, _ = json.Marshal(payload)
	resp, err = http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST with options failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 for valid question, got %d", resp.StatusCode)
	}
}

func TestHandlers_ResolveActivity_Validation(t *testing.T) {
	s, baseURL := setupTestServer(t)
	defer s.Stop()

	_, delID := setupTestData(t, baseURL)

	// Create an activity first
	payload := map[string]interface{}{
		"type":    "decision",
		"content": "Choose framework",
	}
	body, _ := json.Marshal(payload)
	resp, _ := http.Post(
		fmt.Sprintf("%s/api/delegations/%s/activities", baseURL, delID),
		"application/json",
		bytes.NewReader(body),
	)
	var activity ActivityEvent
	_ = json.NewDecoder(resp.Body).Decode(&activity)
	resp.Body.Close()

	// Test PATCH 400 — missing resolution field
	resolvePayload := map[string]string{}
	resolveBody, _ := json.Marshal(resolvePayload)
	resolveURL := fmt.Sprintf("%s/api/delegations/%s/activities/%s", baseURL, delID, activity.ID)
	req, _ := http.NewRequest(http.MethodPatch, resolveURL, bytes.NewReader(resolveBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH missing resolution failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing resolution, got %d", resp.StatusCode)
	}

	// Test PATCH 400 — bad JSON
	req, _ = http.NewRequest(http.MethodPatch, resolveURL, strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH bad JSON failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", resp.StatusCode)
	}
}

// --- Permission Frontmatter Helper Tests ---

func TestExtractPermissionsFromFrontmatter_NewFormat(t *testing.T) {
	fm := `description: Test agent
mode: all
permission:
  read: allow
  edit: deny
  write: deny
  bash: allow
  glob: allow
`
	perms := extractPermissionsFromFrontmatter(fm)
	if perms["read"] != "allow" {
		t.Errorf("expected read=allow, got %q", perms["read"])
	}
	if perms["edit"] != "deny" {
		t.Errorf("expected edit=deny, got %q", perms["edit"])
	}
	if perms["bash"] != "allow" {
		t.Errorf("expected bash=allow, got %q", perms["bash"])
	}
	if len(perms) != 5 {
		t.Errorf("expected 5 permissions, got %d", len(perms))
	}
}

func TestExtractPermissionsFromFrontmatter_OldFormat(t *testing.T) {
	fm := `description: Test agent
mode: all
tools:
  read: true
  edit: false
  write: false
  bash: true
`
	perms := extractPermissionsFromFrontmatter(fm)
	if perms["read"] != "allow" {
		t.Errorf("expected read=allow, got %q", perms["read"])
	}
	if perms["edit"] != "deny" {
		t.Errorf("expected edit=deny, got %q", perms["edit"])
	}
	if perms["write"] != "deny" {
		t.Errorf("expected write=deny, got %q", perms["write"])
	}
	if perms["bash"] != "allow" {
		t.Errorf("expected bash=allow, got %q", perms["bash"])
	}
}

func TestExtractPermissionsFromFrontmatter_Empty(t *testing.T) {
	fm := `description: Test agent
mode: all
`
	perms := extractPermissionsFromFrontmatter(fm)
	if len(perms) != 0 {
		t.Errorf("expected 0 permissions, got %d", len(perms))
	}
}

func TestExtractModeFromFrontmatter(t *testing.T) {
	fm := `description: Test agent
mode: primary
permission:
  read: allow
`
	mode := extractModeFromFrontmatter(fm)
	if mode != "primary" {
		t.Errorf("expected mode=primary, got %q", mode)
	}
}

func TestExtractModeFromFrontmatter_Empty(t *testing.T) {
	fm := `description: Test agent
`
	mode := extractModeFromFrontmatter(fm)
	if mode != "" {
		t.Errorf("expected empty mode, got %q", mode)
	}
}

func TestUpdatePermissionsInFrontmatter_ExistingPermission(t *testing.T) {
	content := "---\ndescription: Test agent\nmode: all\npermission:\n  read: allow\n  edit: deny\n---\n\n# Agent Body"
	newPerms := map[string]string{"read": "allow", "edit": "allow", "bash": "ask"}
	updated := updatePermissionsInFrontmatter(content, newPerms)

	// Verify the body is preserved
	if !strings.Contains(updated, "# Agent Body") {
		t.Error("body was lost")
	}

	// Verify new permissions are present
	if !strings.Contains(updated, "  edit: allow") {
		t.Error("edit permission not updated")
	}
	if !strings.Contains(updated, "  bash: ask") {
		t.Error("bash permission not added")
	}

	// Verify old permission block is replaced (not duplicated)
	count := strings.Count(updated, "permission:")
	if count != 1 {
		t.Errorf("expected 1 permission: block, got %d", count)
	}
}

func TestUpdatePermissionsInFrontmatter_OldToolsFormat(t *testing.T) {
	content := "---\ndescription: Test agent\ntools:\n  read: true\n  edit: false\n---\n\n# Agent Body"
	newPerms := map[string]string{"read": "allow", "edit": "allow"}
	updated := updatePermissionsInFrontmatter(content, newPerms)

	// Should replace tools: with permission:
	if strings.Contains(updated, "tools:") {
		t.Error("old tools: block should be replaced")
	}
	if !strings.Contains(updated, "permission:") {
		t.Error("permission: block should exist")
	}
	if !strings.Contains(updated, "  edit: allow") {
		t.Error("edit should be allow")
	}
}

func TestUpdatePermissionsInFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Agent Body\n\nSome content"
	newPerms := map[string]string{"read": "allow"}
	updated := updatePermissionsInFrontmatter(content, newPerms)

	if !strings.HasPrefix(updated, "---") {
		t.Error("should add frontmatter")
	}
	if !strings.Contains(updated, "  read: allow") {
		t.Error("permission should be present")
	}
	if !strings.Contains(updated, "# Agent Body") {
		t.Error("body should be preserved")
	}
}

func TestParseFrontmatter(t *testing.T) {
	content := "---\nfoo: bar\n---\n\nBody text"
	fm, body := parseFrontmatter(content)
	// Frontmatter includes leading/trailing whitespace from between --- delimiters
	if strings.TrimSpace(fm) != "foo: bar" {
		t.Errorf("unexpected frontmatter: %q", fm)
	}
	if body != "Body text" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "No frontmatter here"
	fm, body := parseFrontmatter(content)
	if fm != "" {
		t.Errorf("expected empty frontmatter, got %q", fm)
	}
	if body != content {
		t.Errorf("body should equal content")
	}
}

// --- Permission validation tests ---

func TestValidPermissionKeys_AllKnownKeys(t *testing.T) {
	expected := []string{
		"read", "edit", "write", "bash", "glob", "grep", "lsp", "ast_grep",
		"websearch", "code_search", "webfetch",
		"task", "delegate", "question", "skill",
		"memory", "intercom", "ado", "mcp",
	}
	for _, key := range expected {
		if !ValidPermissionKeys[key] {
			t.Errorf("expected key %q to be valid", key)
		}
	}
}

func TestValidPermissionKeys_RejectsInvalid(t *testing.T) {
	invalid := []string{"unknown_tool", "sudo", "admin", "root", "exec", "",
		"todowrite", "todoread", "delegation_list", "delegation_read"} // deprecated keys
	for _, key := range invalid {
		if ValidPermissionKeys[key] {
			t.Errorf("key %q should NOT be valid", key)
		}
	}
}

func TestValidPermissionValues_AllKnown(t *testing.T) {
	for _, v := range []string{"allow", "ask", "deny"} {
		if !ValidPermissionValues[v] {
			t.Errorf("expected value %q to be valid", v)
		}
	}
}

func TestValidPermissionValues_RejectsInvalid(t *testing.T) {
	invalid := []string{"yes", "no", "true", "false", "1", "0", "ENABLED", ""}
	for _, v := range invalid {
		if ValidPermissionValues[v] {
			t.Errorf("value %q should NOT be valid", v)
		}
	}
}

func TestSortedPermissionKeys_IsSorted(t *testing.T) {
	keys := sortedPermissionKeys()
	if len(keys) != len(ValidPermissionKeys) {
		t.Errorf("expected %d keys, got %d", len(ValidPermissionKeys), len(keys))
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] >= keys[i] {
			t.Errorf("keys not sorted: %q >= %q", keys[i-1], keys[i])
		}
	}
}

func TestPutAgentPermissions_SyncsFrontmatter(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	opencodeJSON := `{
		"agent": {
			"test": {
				"mode": "subagent"
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(configDir, "opencode.json"), []byte(opencodeJSON), 0o644); err != nil {
		t.Fatalf("write opencode.json: %v", err)
	}

	agentsDir := filepath.Join(configDir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	mdContent := `---
description: Test agent
mode: subagent
permission:
  read: allow
  edit: allow
---
Prompt body
`
	mdPath := filepath.Join(agentsDir, "test.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatalf("write agent md: %v", err)
	}

	s, baseURL := setupTestServer(t)
	defer s.Stop()

	payload := map[string]string{
		"read": "allow",
		"edit": "deny",
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/config/agents/test/permissions", baseURL), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update permissions failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	data, err := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	var cfg struct {
		Agent map[string]struct {
			Permission map[string]string `json:"permission"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse opencode.json: %v", err)
	}
	if cfg.Agent["test"].Permission["edit"] != "deny" {
		t.Errorf("opencode.json permission not updated: got %q", cfg.Agent["test"].Permission["edit"])
	}

	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read agent md: %v", err)
	}
	if !strings.Contains(string(mdData), "edit: deny") {
		t.Errorf("AGENT.md frontmatter not updated: %s", string(mdData))
	}
	if !strings.Contains(string(mdData), "Prompt body") {
		t.Errorf("AGENT.md body was lost: %s", string(mdData))
	}
}
