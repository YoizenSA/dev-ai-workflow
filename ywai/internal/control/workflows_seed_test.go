package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// seedWorkflowJSON returns the real bundled seed for name, or skips the test
// when no seed source is available (the embedded FS is absent in test builds).
func seedWorkflowJSON(t *testing.T, name string) []byte {
	t.Helper()
	raw, ok := config.SeedWorkflowJSON(name)
	if !ok {
		t.Skipf("no bundled seed available for %q (no embedded FS, no source checkout)", name)
	}
	return raw
}

func TestSeedDiffAndApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mux := http.NewServeMux()
	srv := &Server{mux: mux}
	srv.registerWorkflowsRoutes()
	srv.workflows.store = storeAt(t, filepath.Join(home, ".ywai", "workflows"))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	seed := seedWorkflowJSON(t, "planning")

	// Install a stale, shrunken copy of the seed workflow.
	var seedWF map[string]any
	if err := json.Unmarshal(seed, &seedWF); err != nil {
		t.Fatalf("parse seed: %v", err)
	}
	stale := `{"id":"planning","name":"planning","version":"0.9.0",
		"nodes":[
			{"id":"start","type":"start","name":"start","position":{"x":0,"y":0},"data":{"label":"Start"}},
			{"id":"end","type":"end","name":"end","position":{"x":1,"y":0},"data":{"label":"End"}}
		],"connections":[]}`
	wfDir := filepath.Join(home, ".ywai", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "planning.json"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. List flags the drift against the seed.
	resp := mustDo(t, client, "GET", server.URL+"/api/workflows", "")
	body := readBody(resp)
	t.Logf("list body: %s", body)
	var list struct {
		Workflows []struct {
			Name string `json:"name"`
			Seed *struct {
				InSync     bool     `json:"inSync"`
				Version    string   `json:"version"`
				AddedNodes []string `json:"addedNodes"`
			} `json:"seed"`
		} `json:"workflows"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	var item *struct {
		Name string `json:"name"`
		Seed *struct {
			InSync     bool     `json:"inSync"`
			Version    string   `json:"version"`
			AddedNodes []string `json:"addedNodes"`
		} `json:"seed"`
	}
	for i := range list.Workflows {
		if list.Workflows[i].Name == "planning" {
			item = &list.Workflows[i]
		}
	}
	if item == nil || item.Seed == nil {
		t.Fatalf("planning row with seed info missing: %s", body)
	}
	if item.Seed.InSync {
		t.Fatal("stale copy should not be in sync with the seed")
	}
	if len(item.Seed.AddedNodes) == 0 {
		t.Fatalf("expected added nodes in the diff: %+v", item.Seed)
	}

	// 2. Apply overwrites the design with the seed and keeps a backup.
	resp = mustDo(t, client, "POST", server.URL+"/api/workflows/planning/seed-apply", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("seed-apply: %d %s", resp.StatusCode, readBody(resp))
	}
	backups, err := filepath.Glob(filepath.Join(home, ".ywai", "workflows", "backups", "planning-*.json"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("expected exactly one backup, got %v (%v)", backups, err)
	}
	if raw, err := os.ReadFile(backups[0]); err != nil || string(raw) != stale {
		t.Fatalf("backup should hold the pre-apply design: %v", err)
	}

	// 3. After applying, the workflow is in sync with the seed.
	resp = mustDo(t, client, "GET", server.URL+"/api/workflows", "")
	if !containsInSyncPlanning(resp) {
		t.Fatalf("planning should be in sync after apply: %s", readBody(resp))
	}
}

// containsInSyncPlanning re-reads the list body and reports whether the
// planning row carries seed.inSync == true.
func containsInSyncPlanning(resp *http.Response) bool {
	defer resp.Body.Close()
	var list struct {
		Workflows []struct {
			Name string `json:"name"`
			Seed *struct {
				InSync bool `json:"inSync"`
			} `json:"seed"`
		} `json:"workflows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return false
	}
	for _, w := range list.Workflows {
		if w.Name == "planning" {
			return w.Seed != nil && w.Seed.InSync
		}
	}
	return false
}

func TestSeedApplyUnknownWorkflow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mux := http.NewServeMux()
	srv := &Server{mux: mux}
	srv.registerWorkflowsRoutes()
	srv.workflows.store = storeAt(t, filepath.Join(home, ".ywai", "workflows"))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	// A name that can never have a bundled seed.
	resp := mustDo(t, client, "POST", server.URL+"/api/workflows/zzz-no-such/seed-apply", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing seed, got %d: %s", resp.StatusCode, readBody(resp))
	}
}
