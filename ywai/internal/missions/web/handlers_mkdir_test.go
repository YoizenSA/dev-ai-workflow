package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkdirBody builds a JSON request body for the mkdir endpoint.
// Uses json.Marshal to properly escape backslashes on Windows.
func mkdirBody(parentPath, name string) string {
	b, err := json.Marshal(map[string]string{
		"parentPath": parentPath,
		"name":       name,
	})
	if err != nil {
		panic(fmt.Sprintf("mkdirBody: %v", err))
	}
	return string(b)
}

func TestMkdirFS(t *testing.T) {
	parent := t.TempDir()

	cases := []struct {
		name       string
		body       string
		wantStatus int
		// for success cases: the folder that should exist on disk afterwards
		wantDir string
		// substring expected in the JSON response body ("" = skip check)
		wantBody string
		// precondition: create this folder before the test runs (relative to parent)
		preCreate string
	}{
		{
			name:       "valid single segment",
			body:       mkdirBody(parent, "newproj"),
			wantStatus: http.StatusCreated,
			wantDir:    filepath.Join(parent, "newproj"),
			wantBody:   func() string { b, _ := json.Marshal(filepath.Join(parent, "newproj")); return `"path":` + string(b) }(),
		},
		{
			name:       "empty name",
			body:       mkdirBody(parent, "  "),
			wantStatus: http.StatusBadRequest,
			wantBody:   "folder name is required",
		},
		{
			name:       "name with path separator is rejected",
			body:       mkdirBody(parent, "a/b"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "dotdot as name rejected",
			body:       mkdirBody(parent, ".."),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "already exists",
			body:       mkdirBody(parent, "exists"),
			wantStatus: http.StatusConflict,
			wantBody:   "already exists",
			preCreate:  "exists",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.preCreate != "" {
				if err := os.Mkdir(filepath.Join(parent, tc.preCreate), 0755); err != nil {
					t.Fatalf("precreate: %v", err)
				}
			}

			h := &Handlers{}
			req := httptest.NewRequest(http.MethodPost, "/api/fs/mkdir", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.MkdirFS(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantDir != "" {
				info, err := os.Stat(tc.wantDir)
				if err != nil {
					t.Fatalf("expected dir %s to exist: %v", tc.wantDir, err)
				}
				if !info.IsDir() {
					t.Fatalf("%s is not a directory", tc.wantDir)
				}
			}
		})
	}
}

// TestMkdirFS_DefaultParent exercises the empty-parentPath branch (defaults to home).
// Kept separate because it depends on os.UserHomeDir().
func TestMkdirFS_DefaultParent(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home dir: %v", err)
	}
	// Use a unique name to avoid colliding with real user data.
	dirName := "ywai-mkdir-test-" + filepath.Base(t.Name())
	defer os.RemoveAll(filepath.Join(home, dirName))

	h := &Handlers{}
	body := `{"name":"` + dirName + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/fs/mkdir", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.MkdirFS(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Path != filepath.Join(home, dirName) {
		t.Fatalf("path = %q, want %q", resp.Path, filepath.Join(home, dirName))
	}
}
