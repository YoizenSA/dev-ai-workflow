package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
)

// useEnvTestRoot pins the profiles dir to a temp dir for one test.
func useEnvTestRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() { envprofile.SetProfilesRootForTest("") })
	envprofile.SetProfilesRootForTest(dir)
	return dir
}

func newEnvTestServer() *Server {
	s := &Server{mux: http.NewServeMux()}
	s.registerEnvProfileRoutes()
	return s
}

func decodeEnvBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v body=%q", err, w.Body.String())
	}
	return out
}

func TestHandleEnvList_Empty(t *testing.T) {
	useEnvTestRoot(t)
	s := newEnvTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/envs", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := decodeEnvBody(t, w)
	envs, _ := body["envs"].([]any)
	if len(envs) != 0 {
		t.Fatalf("envs=%v want empty", envs)
	}
	if _, ok := body["presets"]; !ok {
		t.Fatalf("missing presets key body=%v", body)
	}
}

func TestHandleEnvCRUD(t *testing.T) {
	useEnvTestRoot(t)
	s := newEnvTestServer()

	// Create.
	req := httptest.NewRequest(http.MethodPost, "/api/envs", strings.NewReader(`{"name":"dev","preset":"dev"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	// Duplicate is 409.
	dup := httptest.NewRequest(http.MethodPost, "/api/envs", strings.NewReader(`{"name":"dev"}`))
	dup.Header.Set("Content-Type", "application/json")
	dw := httptest.NewRecorder()
	s.mux.ServeHTTP(dw, dup)
	if dw.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d want 409 body=%s", dw.Code, dw.Body.String())
	}

	// List shows preset/port/status.
	lreq := httptest.NewRequest(http.MethodGet, "/api/envs", nil)
	lw := httptest.NewRecorder()
	s.mux.ServeHTTP(lw, lreq)
	if lw.Code != http.StatusOK {
		t.Fatalf("list status=%d", lw.Code)
	}
	lbody := decodeEnvBody(t, lw)
	envs, _ := lbody["envs"].([]any)
	if len(envs) != 1 {
		t.Fatalf("envs len=%d want 1", len(envs))
	}
	first, _ := envs[0].(map[string]any)
	for _, k := range []string{"name", "preset", "port", "running", "url"} {
		if _, ok := first[k]; !ok {
			t.Errorf("list item missing %q: %v", k, first)
		}
	}

	// Patch preset dev -> qa.
	preq := httptest.NewRequest(http.MethodPatch, "/api/envs/dev", strings.NewReader(`{"preset":"qa"}`))
	preq.SetPathValue("name", "dev")
	preq.Header.Set("Content-Type", "application/json")
	pw := httptest.NewRecorder()
	s.mux.ServeHTTP(pw, preq)
	if pw.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", pw.Code, pw.Body.String())
	}
	if got, err := envprofile.Get("dev"); err != nil || got.Preset != "qa" {
		t.Fatalf("manifest preset=%v err=%v", got, err)
	}

	// Status carries service/db/auth/doctor.
	sreq := httptest.NewRequest(http.MethodGet, "/api/envs/dev/status", nil)
	sreq.SetPathValue("name", "dev")
	sw := httptest.NewRecorder()
	s.mux.ServeHTTP(sw, sreq)
	if sw.Code != http.StatusOK {
		t.Fatalf("status code=%d body=%s", sw.Code, sw.Body.String())
	}
	sbody := decodeEnvBody(t, sw)
	for _, k := range []string{"service", "db", "auth", "doctor", "url"} {
		if _, ok := sbody[k]; !ok {
			t.Errorf("status missing %q", k)
		}
	}

	// Logs on a fresh profile: no file yet, empty lines.
	greq := httptest.NewRequest(http.MethodGet, "/api/envs/dev/logs", nil)
	greq.SetPathValue("name", "dev")
	gw := httptest.NewRecorder()
	s.mux.ServeHTTP(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("logs status=%d", gw.Code)
	}

	// Delete.
	dreq := httptest.NewRequest(http.MethodDelete, "/api/envs/dev", nil)
	dreq.SetPathValue("name", "dev")
	drw := httptest.NewRecorder()
	s.mux.ServeHTTP(drw, dreq)
	if drw.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", drw.Code, drw.Body.String())
	}
	if envprofile.Exists("dev") {
		t.Fatalf("dev still exists after delete")
	}
}

func TestHandleEnvCreate_Validation(t *testing.T) {
	useEnvTestRoot(t)
	s := newEnvTestServer()
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"empty name", `{"name":""}`, http.StatusBadRequest},
		{"bad chars", `{"name":"Dev!"}`, http.StatusBadRequest},
		{"reserved", `{"name":"install"}`, http.StatusBadRequest},
		{"bad json", `not json`, http.StatusBadRequest},
		{"unknown preset", `{"name":"dev","preset":"nope"}`, http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/envs", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%s: status=%d want %d body=%s", tc.name, w.Code, tc.want, w.Body.String())
		}
	}
}

func TestHandleEnvSingle_NotFound(t *testing.T) {
	useEnvTestRoot(t)
	s := newEnvTestServer()

	cases := []struct {
		method string
		target string
	}{
		{http.MethodPatch, "/api/envs/ghost"},
		{http.MethodDelete, "/api/envs/ghost"},
		{http.MethodGet, "/api/envs/ghost/status"},
		{http.MethodGet, "/api/envs/ghost/logs"},
		{http.MethodPost, "/api/envs/ghost/apply"},
	}
	for _, tc := range cases {
		var body *strings.Reader
		if tc.method == http.MethodPatch {
			body = strings.NewReader(`{"preset":"qa"}`)
		} else {
			body = strings.NewReader("")
		}
		req := httptest.NewRequest(tc.method, tc.target, body)
		req.SetPathValue("name", "ghost")
		if tc.method == http.MethodPatch {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: status=%d want 404 body=%s", tc.method, tc.target, w.Code, w.Body.String())
		}
	}
}

func TestHandleEnvPatch_UnknownPreset(t *testing.T) {
	useEnvTestRoot(t)
	if _, err := envprofile.Create("dev", "dev"); err != nil {
		t.Fatal(err)
	}
	s := newEnvTestServer()
	req := httptest.NewRequest(http.MethodPatch, "/api/envs/dev", strings.NewReader(`{"preset":"nope"}`))
	req.SetPathValue("name", "dev")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", w.Code, w.Body.String())
	}
}

func TestHandleEnvApply_UsesSubprocessSeam(t *testing.T) {
	useEnvTestRoot(t)
	if _, err := envprofile.Create("dev", "dev"); err != nil {
		t.Fatal(err)
	}
	// Stub the subprocess runner: the handler must call it and must not
	// mutate this process env (no in-process sandbox).
	old := envApplyRunner
	defer func() { envApplyRunner = old }()
	var sawName string
	envApplyRunner = func(ctx context.Context, name string) (string, error) {
		sawName = name
		if got := os.Getenv("YWAI_PROFILE"); got != "" {
			t.Errorf("YWAI_PROFILE=%q during apply, want empty (no in-process sandbox)", got)
		}
		return "fake install ok", nil
	}
	s := newEnvTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/envs/dev/apply", nil)
	req.SetPathValue("name", "dev")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if sawName != "dev" {
		t.Fatalf("runner saw %q want dev", sawName)
	}
	body := decodeEnvBody(t, w)
	if body["output"] != "fake install ok" {
		t.Errorf("output=%v want stub output", body["output"])
	}
	if os.Getenv("YWAI_PROFILE") != "" {
		t.Errorf("handler leaked YWAI_PROFILE=%q", os.Getenv("YWAI_PROFILE"))
	}
}

func TestHandleEnvLogs_Tail(t *testing.T) {
	dir := useEnvTestRoot(t)
	p, err := envprofile.Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	_ = dir
	// Write a 5-line log through the real profile log path.
	logPath := envprofile.ServerLog(p)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("l1\nl2\nl3\nl4\nl5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newEnvTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/envs/dev/logs?lines=2", nil)
	req.SetPathValue("name", "dev")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var body struct {
		Lines     []string `json:"lines"`
		Total     int      `json:"total"`
		Truncated bool     `json:"truncated"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Lines) != 2 || body.Lines[0] != "l4" || body.Lines[1] != "l5" {
		t.Errorf("lines=%v want [l4 l5]", body.Lines)
	}
	if body.Total != 5 || !body.Truncated {
		t.Errorf("total=%d truncated=%v want 5 true", body.Total, body.Truncated)
	}
}
