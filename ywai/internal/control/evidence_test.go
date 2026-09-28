package control

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newEvidenceServer mounts only the evidence routes and returns the server
// plus a fresh project directory to point ?project_dir= at.
func newEvidenceServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	projectDir := t.TempDir()
	mux := http.NewServeMux()
	srv := &Server{mux: mux}
	srv.registerEvidenceRoutes()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, projectDir
}

// writeEvidenceRun fabricates one .evidence/<runID>/ folder with the given
// files in the project directory.
func writeEvidenceRun(t *testing.T, projectDir, runID string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(projectDir, ".evidence", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEvidenceList(t *testing.T) {
	server, projectDir := newEvidenceServer(t)
	writeEvidenceRun(t, projectDir, "run-old", map[string]string{"report.md": "old", "old.png": "x"})
	writeEvidenceRun(t, projectDir, "run-new", map[string]string{"report.md": "new", "a.png": "x", "b.log": "y"})
	// Make the ordering deterministic: run-new is the newest.
	newer := time.Now()
	older := newer.Add(-time.Hour)
	for run, mt := range map[string]time.Time{"run-new": newer, "run-old": older} {
		path := filepath.Join(projectDir, ".evidence", run)
		if err := os.Chtimes(path, mt, mt); err != nil {
			t.Fatal(err)
		}
	}

	resp := mustDo(t, server.Client(), "GET", server.URL+"/api/evidence?project_dir="+projectDir, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d %s", resp.StatusCode, readBody(resp))
	}
	body := readBody(resp)
	for _, want := range []string{`"runId":"run-new"`, `"runId":"run-old"`, `"files":3`, `"files":2`} {
		if !strings.Contains(body, want) {
			t.Errorf("list body missing %s:\n%s", want, body)
		}
	}
	// Newest first.
	if strings.Index(body, "run-new") > strings.Index(body, "run-old") {
		t.Errorf("list should be newest first:\n%s", body)
	}
}

func TestEvidenceRunDetail(t *testing.T) {
	server, projectDir := newEvidenceServer(t)
	writeEvidenceRun(t, projectDir, "run-1", map[string]string{
		"report.md": "# Run report\nAll green.",
		"login.png": "png-bytes",
	})

	resp := mustDo(t, server.Client(), "GET", server.URL+"/api/evidence/run-1?project_dir="+projectDir, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detail: %d %s", resp.StatusCode, readBody(resp))
	}
	body := readBody(resp)
	for _, want := range []string{`"runId":"run-1"`, "All green", `"name":"login.png"`, `"name":"report.md"`} {
		if !strings.Contains(body, want) {
			t.Errorf("detail body missing %q:\n%s", want, body)
		}
	}

	// Unknown run id (valid slug) is a 404.
	resp = mustDo(t, server.Client(), "GET", server.URL+"/api/evidence/nope?project_dir="+projectDir, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown run should 404, got %d", resp.StatusCode)
	}
}

func TestEvidenceTraversalAndBadSlug(t *testing.T) {
	server, projectDir := newEvidenceServer(t)
	writeEvidenceRun(t, projectDir, "run-1", map[string]string{"report.md": "x"})

	// Encoded traversal escapes .evidence -> 403 (rejected by containment
	// before anything on disk is touched).
	resp := mustDo(t, server.Client(), "GET", server.URL+"/api/evidence/..%2F..%2Fetc?project_dir="+projectDir, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("traversal should 403, got %d: %s", resp.StatusCode, readBody(resp))
	}
	// Bad slug (uppercase) -> 400.
	resp = mustDo(t, server.Client(), "GET", server.URL+"/api/evidence/RUN-1?project_dir="+projectDir, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad slug should 400, got %d: %s", resp.StatusCode, readBody(resp))
	}
}

func TestEvidenceListEmptyOrMissing(t *testing.T) {
	server, projectDir := newEvidenceServer(t)

	// Empty .evidence dir -> [].
	if err := os.MkdirAll(filepath.Join(projectDir, ".evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	resp := mustDo(t, server.Client(), "GET", server.URL+"/api/evidence?project_dir="+projectDir, "")
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(readBody(resp)) != "[]" {
		t.Errorf("empty .evidence should 200 [], got %d %s", resp.StatusCode, readBody(resp))
	}

	// Missing .evidence dir -> [] too (never a 500).
	missing := t.TempDir()
	resp = mustDo(t, server.Client(), "GET", server.URL+"/api/evidence?project_dir="+missing, "")
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(readBody(resp)) != "[]" {
		t.Errorf("missing .evidence should 200 [], got %d %s", resp.StatusCode, readBody(resp))
	}
}

// TestEvidenceProjectDirFallback covers the os.Getwd() fallback: no
// ?project_dir= still answers 200 with [] for a cwd without .evidence.
func TestEvidenceProjectDirFallback(t *testing.T) {
	empty := t.TempDir()
	t.Chdir(empty)
	server, _ := newEvidenceServer(t)

	resp := mustDo(t, server.Client(), "GET", server.URL+"/api/evidence", "")
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(readBody(resp)) != "[]" {
		t.Errorf("cwd fallback should 200 [], got %d %s", resp.StatusCode, readBody(resp))
	}
}
