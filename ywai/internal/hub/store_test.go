package hub

import (
	"context"
	"fmt"
	"testing"
)

func TestStore_AddProject(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p := Project{
		ID:          "proj-1",
		Name:        "Test Project",
		Path:        "/tmp/test-project",
		AgentType:   "opencode",
		SyncEnabled: true,
	}

	err := store.AddProject(ctx, p)
	if err != nil {
		t.Fatalf("AddProject error = %v", err)
	}
}

func TestStore_GetProject_Found(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p := Project{
		ID:   "proj-1",
		Name: "Test Project",
		Path: "/tmp/test-project",
	}
	if err := store.AddProject(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetProject error = %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("GetProject ID = %q, want %q", got.ID, p.ID)
	}
	if got.Name != p.Name {
		t.Errorf("GetProject Name = %q, want %q", got.Name, p.Name)
	}
	if got.Path != p.Path {
		t.Errorf("GetProject Path = %q, want %q", got.Path, p.Path)
	}
}

func TestStore_GetProject_NotFound(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	_, err := store.GetProject(ctx, "nonexistent")
	if err == nil {
		t.Fatal("GetProject expected error for unknown ID, got nil")
	}
}

func TestStore_AddProject_DuplicatePath(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p1 := Project{
		ID:   "proj-1",
		Name: "Project One",
		Path: "/tmp/same-path",
	}
	p2 := Project{
		ID:   "proj-2",
		Name: "Project Two",
		Path: "/tmp/same-path",
	}

	if err := store.AddProject(ctx, p1); err != nil {
		t.Fatal(err)
	}
	err := store.AddProject(ctx, p2)
	if err == nil {
		t.Fatal("AddProject expected error for duplicate path, got nil")
	}
}

func TestStore_ListProjects_Empty(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects error = %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("ListProjects returned %d projects, want 0", len(projects))
	}
}

func TestStore_ListProjects_Multiple(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p1 := Project{ID: "proj-1", Name: "Alpha", Path: "/tmp/alpha"}
	p2 := Project{ID: "proj-2", Name: "Beta", Path: "/tmp/beta"}
	p3 := Project{ID: "proj-3", Name: "Gamma", Path: "/tmp/gamma"}

	for _, p := range []Project{p1, p2, p3} {
		if err := store.AddProject(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects error = %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("ListProjects returned %d projects, want 3", len(projects))
	}

	ids := make(map[string]bool)
	for _, p := range projects {
		ids[p.ID] = true
	}
	for _, id := range []string{"proj-1", "proj-2", "proj-3"} {
		if !ids[id] {
			t.Errorf("ListProjects missing project ID %q", id)
		}
	}
}

func TestStore_RemoveProject(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p := Project{ID: "proj-1", Name: "To Remove", Path: "/tmp/to-remove"}
	if err := store.AddProject(ctx, p); err != nil {
		t.Fatal(err)
	}

	err := store.RemoveProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("RemoveProject error = %v", err)
	}

	// Removal is idempotent — removing again should not error
	err = store.RemoveProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("RemoveProject second call error = %v (should be idempotent)", err)
	}

	_, err = store.GetProject(ctx, "proj-1")
	if err == nil {
		t.Error("GetProject after RemoveProject expected error, got nil")
	}
}

func TestStore_AddProject_EmptyID(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p := Project{
		ID:   "",
		Name: "No ID",
		Path: "/tmp/no-id",
	}

	err := store.AddProject(ctx, p)
	if err == nil {
		t.Fatal("AddProject expected error for empty ID, got nil")
	}
}

func TestStore_AddProject_EmptyPath(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p := Project{
		ID:   "proj-1",
		Name: "No Path",
		Path: "",
	}

	err := store.AddProject(ctx, p)
	if err == nil {
		t.Fatal("AddProject expected error for empty path, got nil")
	}
}

func TestStore_AddProject_DuplicateID(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p1 := Project{
		ID:   "proj-1",
		Name: "First",
		Path: "/tmp/first",
	}
	p2 := Project{
		ID:   "proj-1",
		Name: "Second",
		Path: "/tmp/second",
	}

	if err := store.AddProject(ctx, p1); err != nil {
		t.Fatal(err)
	}
	err := store.AddProject(ctx, p2)
	if err == nil {
		t.Fatal("AddProject expected error for duplicate ID, got nil")
	}
}

func TestStore_RemoveProject_Nonexistent(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	err := store.RemoveProject(ctx, "never-added")
	if err != nil {
		t.Fatalf("RemoveProject for nonexistent ID should be idempotent, got error = %v", err)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	proj := Project{
		ID:   "concurrent-1",
		Name: "Concurrent",
		Path: "/tmp/concurrent",
	}

	if err := store.AddProject(ctx, proj); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			store.ListProjects(ctx)
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			store.GetProject(ctx, "concurrent-1")
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			p := Project{
				ID:   fmt.Sprintf("concurrent-%d", i+2),
				Name: "Concurrent",
				Path: fmt.Sprintf("/tmp/concurrent-%d", i+2),
			}
			store.AddProject(ctx, p)
		}
		done <- struct{}{}
	}()

	for i := 0; i < 3; i++ {
		<-done
	}

	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects after concurrent access error = %v", err)
	}
	if len(projects) < 1 {
		t.Error("ListProjects returned 0 projects after concurrent adds")
	}
}

func TestStore_UpdateProject(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	p := Project{ID: "proj-1", Name: "Original", Path: "/tmp/original"}
	if err := store.AddProject(ctx, p); err != nil {
		t.Fatal(err)
	}

	// Update name and path
	updated, err := store.UpdateProject(ctx, "proj-1", Project{
		Name: "Updated",
		Path: "/tmp/updated",
	})
	if err != nil {
		t.Fatalf("UpdateProject error = %v", err)
	}
	if updated.ID != "proj-1" {
		t.Errorf("updated ID = %q, want %q", updated.ID, "proj-1")
	}
	if updated.Name != "Updated" {
		t.Errorf("updated Name = %q, want %q", updated.Name, "Updated")
	}
	if updated.Path != "/tmp/updated" {
		t.Errorf("updated Path = %q, want %q", updated.Path, "/tmp/updated")
	}
	if updated.UpdatedAt.IsZero() {
		t.Error("updated project has zero UpdatedAt")
	}
	if !updated.UpdatedAt.After(p.CreatedAt) && !updated.UpdatedAt.Equal(p.CreatedAt) {
		t.Error("UpdatedAt should be after or equal to CreatedAt")
	}

	// Verify store reflects update
	got, err := store.GetProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetProject after update error = %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("GetProject after update Name = %q, want %q", got.Name, "Updated")
	}
}

func TestStore_UpdateProject_NotFound(t *testing.T) {
	store := NewProjectStore()
	ctx := context.Background()

	_, err := store.UpdateProject(ctx, "nonexistent", Project{
		Name: "Ghost",
		Path: "/tmp/ghost",
	})
	if err == nil {
		t.Fatal("UpdateProject expected error for nonexistent project, got nil")
	}
}


