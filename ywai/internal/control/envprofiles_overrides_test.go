package control

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
)

func patchEnv(t *testing.T, s *Server, name, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/envs/"+name, strings.NewReader(body))
	req.SetPathValue("name", name)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH %s status=%d body=%s", body, w.Code, w.Body.String())
	}
}

// An emptied list must persist as "install none" and Reset must restore
// inheritance — both were lost before (omitempty + null-as-absent).
func TestHandleEnvPatch_EmptyListAndReset(t *testing.T) {
	useEnvTestRoot(t)
	if _, err := envprofile.Create("dev", "dev"); err != nil {
		t.Fatal(err)
	}
	s := newEnvTestServer()

	patchEnv(t, s, "dev", `{"skills":[]}`)
	p, err := envprofile.Get("dev")
	if err != nil {
		t.Fatal(err)
	}
	if p.Overrides == nil || p.Overrides.Skills == nil || len(p.Overrides.Skills) != 0 {
		t.Fatalf("empty skills override not persisted: %+v", p.Overrides)
	}
	spec, _ := envprofile.PresetSpec(p)
	if !envprofile.PresetNone(spec, "skills") {
		t.Errorf("persisted empty skills must mean install none")
	}

	patchEnv(t, s, "dev", `{"reset":["skills"]}`)
	p, _ = envprofile.Get("dev")
	if p.Overrides != nil {
		t.Errorf("reset of the only override must clear overrides, got %+v", p.Overrides)
	}
}
