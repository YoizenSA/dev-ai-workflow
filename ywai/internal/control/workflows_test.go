package control

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

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
)

func TestWorkflowsE2E_CreateValidateExport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mux := http.NewServeMux()
	srv := &Server{mux: mux}
	srv.registerWorkflowsRoutes()
	// Re-point store + exporter at temp dirs.
	srv.workflows.store = storeAt(t, filepath.Join(home, ".ywai", "workflows"))
	srv.workflows.exporter = exporterAt(t,
		filepath.Join(home, ".config", "opencode", "commands"),
		filepath.Join(home, ".config", "opencode", "agents"),
	)

	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	// 1. Create a workflow.
	body := `{
		"name": "deploy",
		"description": "Deploy flow",
		"version": "1.0.0",
		"nodes": [
			{"id":"s","type":"start","name":"s","position":{"x":0,"y":0},"data":{"label":"Start"}},
			{"id":"b","type":"subAgent","name":"builder","position":{"x":100,"y":0},
			 "data":{"description":"Builds the artifact","agentDefinition":"You build things.","prompt":"Build it.","tools":"read,bash,write"}},
			{"id":"e","type":"end","name":"e","position":{"x":300,"y":0},"data":{"label":"End"}}
		],
		"connections": [
			{"from":"s","to":"b","fromPort":"out","toPort":"input"},
			{"from":"b","to":"e","fromPort":"out","toPort":"in"}
		]
	}`
	resp := mustDo(t, client, "POST", server.URL+"/api/workflows", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, readBody(resp))
	}

	// 2. List reflects it.
	resp = mustDo(t, client, "GET", server.URL+"/api/workflows", "")
	if !strings.Contains(readBody(resp), "deploy") {
		t.Fatalf("list should contain deploy: %s", readBody(resp))
	}

	// 3. Validate passes.
	resp = mustDo(t, client, "POST", server.URL+"/api/workflows/deploy/validate", "")
	if !strings.Contains(readBody(resp), `"valid":true`) {
		t.Fatalf("validate should be valid: %s", readBody(resp))
	}

	// 4. Export preview (dry-run) lists the 3 artifacts without writing.
	resp = mustDo(t, client, "POST", server.URL+"/api/workflows/deploy/export", "")
	plan := readBody(resp)
	if !strings.Contains(plan, `"dryRun":true`) {
		t.Fatalf("preview should be dry-run: %s", plan)
	}
	if !strings.Contains(plan, "deploy.md") || !strings.Contains(plan, "deploy-orchestrator.md") || !strings.Contains(plan, "deploy-builder.md") {
		t.Fatalf("preview should list command + orchestrator + builder:\n%s", plan)
	}
	// Nothing on disk yet.
	cmdPath := filepath.Join(home, ".config", "opencode", "commands", "deploy.md")
	if _, err := os.Stat(cmdPath); err == nil {
		t.Fatal("dry-run must not write files")
	}

	// 5. Apply (?apply=true) writes the artifacts.
	resp = mustDo(t, client, "POST", server.URL+"/api/workflows/deploy/export?apply=true", "")
	applyPlan := readBody(resp)
	if !strings.Contains(applyPlan, `"dryRun":false`) {
		t.Fatalf("apply should clear dry-run: %s", applyPlan)
	}
	for _, name := range []string{"commands/deploy.md", "agents/deploy-orchestrator.md", "agents/deploy-builder.md"} {
		p := filepath.Join(home, ".config", "opencode", name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("apply should write %s: %v", name, err)
		}
	}

	// 6. Verify the command content targets the orchestrator.
	content, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("read command: %v", err)
	}
	if !strings.Contains(string(content), "agent: deploy-orchestrator") {
		t.Errorf("command should target orchestrator:\n%s", content)
	}
	if !strings.Contains(string(content), "$ARGUMENTS") {
		t.Errorf("command should pass $ARGUMENTS:\n%s", content)
	}

	// 7. Delete cleans the source (not the exported artifacts — those are opencode's).
	resp = mustDo(t, client, "DELETE", server.URL+"/api/workflows/deploy", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete: %d %s", resp.StatusCode, readBody(resp))
	}
	resp = mustDo(t, client, "GET", server.URL+"/api/workflows/deploy", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted workflow should 404: %d", resp.StatusCode)
	}
}

func TestWorkflowsE2E_Import(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mux := http.NewServeMux()
	srv := &Server{mux: mux}
	srv.registerWorkflowsRoutes()
	srv.workflows.store = storeAt(t, filepath.Join(home, ".ywai", "workflows"))

	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	// Workflow JSON without explicit name derivation needs.
	raw := `{
		"version": "1.0.0",
		"nodes": [
			{"id":"s","type":"start","name":"s","position":{"x":0,"y":0},"data":{"label":"Start"}},
			{"id":"a","type":"subAgent","name":"news","position":{"x":100,"y":0},
			 "data":{"description":"News agent","agentDefinition":"You fetch news.","prompt":"Get news."}},
			{"id":"e","type":"end","name":"e","position":{"x":200,"y":0},"data":{"label":"End"}}
		],
		"connections":[{"from":"s","to":"a"},{"from":"a","to":"e"}]
	}`
	// Import via the wrapper {json, name}.
	body := mustJSON(t, map[string]any{"json": json.RawMessage(raw), "name": "news-flow"})
	resp := mustDo(t, client, "POST", server.URL+"/api/workflows/import", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import: %d %s", resp.StatusCode, readBody(resp))
	}
	out := readBody(resp)
	if !strings.Contains(out, "news-flow") {
		t.Fatalf("import should echo the imported name: %s", out)
	}

	// The workflow is now loadable.
	resp = mustDo(t, client, "GET", server.URL+"/api/workflows/news-flow", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("imported workflow should be loadable: %d", resp.StatusCode)
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

// storeAt returns a workflows.Store rooted at dir (created lazily on write).
func storeAt(t *testing.T, dir string) *workflows.Store {
	t.Helper()
	return workflows.NewStore(dir)
}

// exporterAt returns an Exporter writing to explicit temp dirs.
func exporterAt(t *testing.T, commandsDir, agentsDir string) *workflows.Exporter {
	t.Helper()
	return workflows.NewExporterWithDirs(commandsDir, agentsDir)
}

func mustDo(t *testing.T, c *http.Client, method, url, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return string(b)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
