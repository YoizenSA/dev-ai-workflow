package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPI_ListProjects_Empty(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/hub/projects", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var projects []Project
	if err := json.NewDecoder(rec.Body).Decode(&projects); err != nil {
		t.Fatalf("decode response body error = %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("response body has %d projects, want 0", len(projects))
	}
}

func TestAPI_ListProjects_ReturnsProjects(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	store.AddProject(ctx, Project{
		ID:   "proj-1",
		Name: "Alpha",
		Path: "/tmp/alpha",
	})
	store.AddProject(ctx, Project{
		ID:   "proj-2",
		Name: "Beta",
		Path: "/tmp/beta",
	})

	handler := NewProjectsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/hub/projects", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var projects []Project
	if err := json.NewDecoder(rec.Body).Decode(&projects); err != nil {
		t.Fatalf("decode response body error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("response body has %d projects, want 2", len(projects))
	}

	ids := map[string]bool{}
	for _, p := range projects {
		ids[p.ID] = true
	}
	if !ids["proj-1"] {
		t.Error("response missing project proj-1")
	}
	if !ids["proj-2"] {
		t.Error("response missing project proj-2")
	}
}

func TestAPI_ListProjects_MethodNotAllowed(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/hub/projects", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s status code = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestAPI_CreateProject_Valid(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	body := `{"name":"My Project","path":"/tmp/my-project"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusCreated)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var p Project
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode response body error = %v", err)
	}
	if p.ID == "" {
		t.Error("created project has empty ID")
	}
	if p.Name != "My Project" {
		t.Errorf("project name = %q, want %q", p.Name, "My Project")
	}
	if p.Path != "/tmp/my-project" {
		t.Errorf("project path = %q, want %q", p.Path, "/tmp/my-project")
	}
	if p.CreatedAt.IsZero() {
		t.Error("created project has zero CreatedAt")
	}
	if p.UpdatedAt.IsZero() {
		t.Error("created project has zero UpdatedAt")
	}
}

func TestAPI_CreateProject_InvalidJSON(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	body := `not-json`
	req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPI_CreateProject_MissingName(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	body := `{"path":"/tmp/no-name"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPI_CreateProject_DuplicatePath(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()
	handler := NewProjectsHandler(store)

	store.AddProject(ctx, Project{
		ID:   "existing",
		Name: "Existing",
		Path: "/tmp/existing",
	})

	body := `{"name":"Duplicate","path":"/tmp/existing"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestAPI_GetProject_Found(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()
	handler := NewProjectsHandler(store)

	store.AddProject(ctx, Project{
		ID:   "proj-1",
		Name: "Alpha",
		Path: "/tmp/alpha",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/hub/projects/proj-1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var p Project
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode response body error = %v", err)
	}
	if p.ID != "proj-1" {
		t.Errorf("project ID = %q, want %q", p.ID, "proj-1")
	}
	if p.Name != "Alpha" {
		t.Errorf("project name = %q, want %q", p.Name, "Alpha")
	}
}

func TestAPI_GetProject_NotFound(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/hub/projects/nonexistent", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAPI_UpdateProject_Valid(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()
	handler := NewProjectsHandler(store)

	store.AddProject(ctx, Project{
		ID:   "proj-1",
		Name: "Alpha",
		Path: "/tmp/alpha",
	})

	body := `{"name":"Alpha Updated","path":"/tmp/alpha"}`
	req := httptest.NewRequest(http.MethodPut, "/api/hub/projects/proj-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var p Project
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode response body error = %v", err)
	}
	if p.ID != "proj-1" {
		t.Errorf("project ID = %q, want %q", p.ID, "proj-1")
	}
	if p.Name != "Alpha Updated" {
		t.Errorf("project name = %q, want %q", p.Name, "Alpha Updated")
	}
	if p.UpdatedAt.IsZero() {
		t.Error("updated project has zero UpdatedAt")
	}
}

func TestAPI_UpdateProject_NotFound(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	body := `{"name":"Ghost","path":"/tmp/ghost"}`
	req := httptest.NewRequest(http.MethodPut, "/api/hub/projects/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAPI_UpdateProject_InvalidBody(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()
	handler := NewProjectsHandler(store)

	store.AddProject(ctx, Project{
		ID:   "proj-1",
		Name: "Alpha",
		Path: "/tmp/alpha",
	})

	body := `not-json`
	req := httptest.NewRequest(http.MethodPut, "/api/hub/projects/proj-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPI_DeleteProject_Valid(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()
	handler := NewProjectsHandler(store)

	store.AddProject(ctx, Project{
		ID:   "proj-1",
		Name: "Alpha",
		Path: "/tmp/alpha",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/hub/projects/proj-1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAPI_DeleteProject_NotFound(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/hub/projects/nonexistent", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAPI_DeleteProject_Duplicate(t *testing.T) {
	store := NewProjectStore()
	handler := NewProjectsHandler(store)
	ctx := context.Background()

	store.AddProject(ctx, Project{
		ID:   "proj-1",
		Name: "Alpha",
		Path: "/tmp/alpha",
	})

	// First delete should succeed
	req1 := httptest.NewRequest(http.MethodDelete, "/api/hub/projects/proj-1", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusNoContent {
		t.Errorf("first delete status code = %d, want %d", rec1.Code, http.StatusNoContent)
	}

	// Second delete on same ID should return 404 (project already gone)
	req2 := httptest.NewRequest(http.MethodDelete, "/api/hub/projects/proj-1", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotFound {
		t.Errorf("second delete status code = %d, want %d", rec2.Code, http.StatusNotFound)
	}
}
