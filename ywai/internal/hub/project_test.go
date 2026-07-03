package hub

import (
	"testing"
	"time"
)

func TestProject_ZeroValues(t *testing.T) {
	var p Project
	if p.ID != "" {
		t.Errorf("Project.ID zero value = %q, want empty", p.ID)
	}
	if p.Name != "" {
		t.Errorf("Project.Name zero value = %q, want empty", p.Name)
	}
	if p.Path != "" {
		t.Errorf("Project.Path zero value = %q, want empty", p.Path)
	}
	if p.AgentType != "" {
		t.Errorf("Project.AgentType zero value = %q, want empty", p.AgentType)
	}
	if p.SyncEnabled != false {
		t.Errorf("Project.SyncEnabled zero value = %v, want false", p.SyncEnabled)
	}
	if !p.CreatedAt.IsZero() {
		t.Errorf("Project.CreatedAt zero value = %v, want zero time", p.CreatedAt)
	}
	if !p.UpdatedAt.IsZero() {
		t.Errorf("Project.UpdatedAt zero value = %v, want zero time", p.UpdatedAt)
	}
}

func TestProject_Fields(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	p := Project{
		ID:          "proj-1",
		Name:        "My Project",
		Path:        "/home/user/projects/my-project",
		AgentType:   "opencode",
		SyncEnabled: true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if p.ID != "proj-1" {
		t.Errorf("Project.ID = %q, want %q", p.ID, "proj-1")
	}
	if p.Name != "My Project" {
		t.Errorf("Project.Name = %q, want %q", p.Name, "My Project")
	}
	if p.Path != "/home/user/projects/my-project" {
		t.Errorf("Project.Path = %q, want %q", p.Path, "/home/user/projects/my-project")
	}
	if p.AgentType != "opencode" {
		t.Errorf("Project.AgentType = %q, want %q", p.AgentType, "opencode")
	}
	if p.SyncEnabled != true {
		t.Errorf("Project.SyncEnabled = %v, want true", p.SyncEnabled)
	}
	if !p.CreatedAt.Equal(now) {
		t.Errorf("Project.CreatedAt = %v, want %v", p.CreatedAt, now)
	}
	if !p.UpdatedAt.Equal(now) {
		t.Errorf("Project.UpdatedAt = %v, want %v", p.UpdatedAt, now)
	}
}
