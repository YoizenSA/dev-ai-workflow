package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

func TestResolveEvalEnv(t *testing.T) {
	envs := []config.EvalEnvironment{
		{Name: "local"},
		{Name: "wsl", ServerURL: "http://localhost:4097", DBPath: "\\\\wsl$\\x\\opencode.db"},
	}
	if got := resolveEvalEnv(envs, "wsl"); got.ServerURL != "http://localhost:4097" {
		t.Fatalf("wsl = %+v", got)
	}
	if got := resolveEvalEnv(envs, ""); got.Name != "local" {
		t.Fatalf("empty = %+v, want local default", got)
	}
	if got := resolveEvalEnv(envs, "nope"); got.Name != "local" || got.ServerURL != "" {
		t.Fatalf("unknown = %+v, want zero local", got)
	}
}

func TestEvalServerAndDBPath(t *testing.T) {
	explicit := config.EvalEnvironment{Name: "x", ServerURL: "http://h:1/", DBPath: "d.db"}
	if got := evalServerURL(explicit); got != "http://h:1" {
		t.Fatalf("server = %q, want trimmed", got)
	}
	if got := evalDBPath(explicit); got != "d.db" {
		t.Fatalf("db = %q", got)
	}
	zero := config.EvalEnvironment{Name: "local"}
	if got := evalDBPath(zero); got != "" {
		t.Fatalf("zero db = %q, want empty (historical convention)", got)
	}
	if got := evalServerURL(zero); got == "" {
		t.Fatal("zero server must fall back to default resolution")
	}
}

func TestFilterRunsByEnv(t *testing.T) {
	runs := []evals.Run{
		{ID: "a", Environment: ""},
		{ID: "b", Environment: "local"},
		{ID: "c", Environment: "wsl"},
	}
	if got := filterRunsByEnv(runs, ""); len(got) != 3 {
		t.Fatalf("empty filter = %d, want all", len(got))
	}
	got := filterRunsByEnv(runs, "local")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("local = %+v, want pre-env + local", got)
	}
	if got := filterRunsByEnv(runs, "wsl"); len(got) != 1 || got[0].ID != "c" {
		t.Fatalf("wsl = %+v", got)
	}
	if got := filterRunsByEnv(runs, "nope"); len(got) != 0 {
		t.Fatalf("unknown = %+v, want empty", got)
	}
}

func withEnvs(t *testing.T, envs []config.EvalEnvironment, saved *[][]config.EvalEnvironment) {
	t.Helper()
	oldLoad, oldSave := loadEvalEnvironments, saveEvalEnvironments
	loadEvalEnvironments = func() []config.EvalEnvironment { return envs }
	saveEvalEnvironments = func(next []config.EvalEnvironment) error {
		*saved = append(*saved, append([]config.EvalEnvironment{}, next...))
		envs = next
		loadEvalEnvironments = func() []config.EvalEnvironment { return envs }
		return nil
	}
	t.Cleanup(func() { loadEvalEnvironments, saveEvalEnvironments = oldLoad, oldSave })
}

func TestEvalEnvironmentsCRUD(t *testing.T) {
	var saved [][]config.EvalEnvironment
	withEnvs(t, []config.EvalEnvironment{{Name: "local"}}, &saved)
	srv := &Server{}

	// List defaults.
	rec := httptest.NewRecorder()
	srv.handleEvalEnvironments(rec, httptest.NewRequest(http.MethodGet, "/api/evals/environments", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "local") {
		t.Fatalf("list = %d %q", rec.Code, rec.Body.String())
	}

	// Add.
	rec = httptest.NewRecorder()
	srv.handleSetEvalEnvironment(rec, httptest.NewRequest(http.MethodPost, "/api/evals/environments",
		strings.NewReader(`{"name":"wsl","serverUrl":"http://localhost:4097"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("add = %d %q", rec.Code, rec.Body.String())
	}
	var body map[string][]config.EvalEnvironment
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body["environments"]) != 2 {
		t.Fatalf("add body = %q", rec.Body.String())
	}

	// Empty name rejected.
	rec = httptest.NewRecorder()
	srv.handleSetEvalEnvironment(rec, httptest.NewRequest(http.MethodPost, "/api/evals/environments",
		strings.NewReader(`{"name":""}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty name = %d, want 400", rec.Code)
	}

	// Delete unknown -> 404; delete known -> list without it.
	rec = httptest.NewRecorder()
	srv.handleDeleteEvalEnvironment(rec, httptest.NewRequest(http.MethodDelete, "/api/evals/environments?name=nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown delete = %d, want 404", rec.Code)
	}
	rec = httptest.NewRecorder()
	srv.handleDeleteEvalEnvironment(rec, httptest.NewRequest(http.MethodDelete, "/api/evals/environments?name=wsl", nil))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "wsl") {
		t.Fatalf("delete = %d %q", rec.Code, rec.Body.String())
	}
	if len(saved) != 2 {
		t.Fatalf("saves = %d, want add + delete", len(saved))
	}
}

func TestMergeEvalEnvironments(t *testing.T) {
	registered := []config.EvalEnvironment{
		{Name: "local"},
		{Name: "staging", ServerURL: "http://stage:8080", DBPath: "/tmp/stage.db"},
	}
	profiles := []envprofile.Profile{
		{Name: "dev", Port: 5800},
		{Name: "staging", Port: 5801}, // collision: registered wins
		{Name: "qa", Port: 5802},
	}

	got := mergeEvalEnvironments(registered, profiles)

	if len(got) != 4 {
		t.Fatalf("len = %d, want 4 (local + staging + dev + qa): %+v", len(got), got)
	}
	if got[0].Name != "local" || got[1].Name != "staging" {
		t.Fatalf("registered entries must keep their order: %+v", got)
	}
	if got[1].DBPath != "/tmp/stage.db" {
		t.Errorf("staging DBPath = %q, want unchanged (collision keeps registered)", got[1].DBPath)
	}
	var dev, qa *config.EvalEnvironment
	for i := range got {
		switch got[i].Name {
		case "dev":
			dev = &got[i]
		case "qa":
			qa = &got[i]
		}
	}
	if dev == nil || qa == nil {
		t.Fatalf("profile environments missing: %+v", got)
	}
	if dev.ServerURL != "http://127.0.0.1:5800" {
		t.Errorf("dev ServerURL = %q, want http://127.0.0.1:5800", dev.ServerURL)
	}
	if want := filepath.Join("opencode", "opencode.db"); !strings.Contains(dev.DBPath, want) || !strings.Contains(dev.DBPath, "dev") {
		t.Errorf("dev DBPath = %q, want the profile's opencode database path", dev.DBPath)
	}
	if qa.ServerURL != "http://127.0.0.1:5802" {
		t.Errorf("qa ServerURL = %q, want http://127.0.0.1:5802", qa.ServerURL)
	}
}
