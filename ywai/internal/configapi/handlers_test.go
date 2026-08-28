package configapi

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

// setupTestServer creates a server on a random port, starts it, and returns
// the server and base URL. The caller must defer s.Stop().
func setupTestServer(t *testing.T) (*Server, string) {
	s := New(0)
	go func() {
		if err := s.Start(); err != nil {
			// Server may fail to start (e.g., port conflict); log and skip.
			t.Logf("server Start returned: %v", err)
		}
	}()

	client := &http.Client{Timeout: 1 * time.Second}
	for i := 0; i < 100; i++ {
		port := s.Port()
		if port == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		baseURL := fmt.Sprintf("http://localhost:%d", port)
		resp, err := client.Get(baseURL + "/api/config/agents")
		if err == nil {
			resp.Body.Close()
			return s, baseURL
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server did not start within timeout, last port: %d", s.Port())
	return s, "" // unreachable
}

// --- Permission Frontmatter Helper Tests ---

func TestExtractPermissionsFromFrontmatter_V2Rules(t *testing.T) {
	fm := `description: Test agent
mode: all
permissions:
  - action: read
    resource: "*"
    effect: allow
  - action: edit
    resource: "*"
    effect: deny
  - action: shell
    resource: "*"
    effect: allow
`
	perms := extractPermissionsFromFrontmatter(fm)
	if perms["read"] != "allow" {
		t.Errorf("expected read=allow, got %q", perms["read"])
	}
	if perms["edit"] != "deny" {
		t.Errorf("expected edit=deny, got %q", perms["edit"])
	}
	if perms["bash"] != "allow" {
		t.Errorf("expected bash=allow from shell, got %q", perms["bash"])
	}
}

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
	// bash renders through the same guardrail block the installer uses, so a
	// permissive value arrives as a nested map with the false-green denials
	// attached rather than a bare scalar. One renderer, no drift.
	if !strings.Contains(updated, "  bash:\n") || !strings.Contains(updated, `    "*": ask`) {
		t.Errorf("bash permission not added as a guardrailed block:\n%s", updated)
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
		"memory", "intercom", "mcp",
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
	t.Setenv("USERPROFILE", homeDir)

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

	// Tool permissions are written only to the frontmatter (single source of
	// truth). opencode.json is reserved for the task delegation object and must
	// not gain a scalar tool-permission block from this endpoint.
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
	if cfg.Agent["test"].Permission["edit"] != "" {
		t.Errorf("opencode.json should not carry scalar tool permissions, got edit=%q", cfg.Agent["test"].Permission["edit"])
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
