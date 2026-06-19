package kanban

import (
	"encoding/json"
	"testing"
	"time"
)

func TestActivityTypeConstants(t *testing.T) {
	if ActivityProgress != "progress" {
		t.Errorf("expected ActivityProgress = 'progress', got '%s'", ActivityProgress)
	}
	if ActivityDecision != "decision" {
		t.Errorf("expected ActivityDecision = 'decision', got '%s'", ActivityDecision)
	}
	if ActivityQuestion != "question" {
		t.Errorf("expected ActivityQuestion = 'question', got '%s'", ActivityQuestion)
	}
	if ActivityBlocked != "blocked" {
		t.Errorf("expected ActivityBlocked = 'blocked', got '%s'", ActivityBlocked)
	}
}

func TestActivityEvent_JSONRoundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	resolvedAt := now.Add(time.Hour)

	orig := ActivityEvent{
		ID:           "act-123",
		DelegationID: "del-456",
		Type:         ActivityDecision,
		Content:      "Should we use PostgreSQL?",
		Options:      []string{"yes", "no", "defer"},
		Resolution:   "yes",
		CreatedAt:    now,
		ResolvedAt:   &resolvedAt,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored ActivityEvent
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.ID != orig.ID {
		t.Errorf("ID mismatch: got %q, want %q", restored.ID, orig.ID)
	}
	if restored.DelegationID != orig.DelegationID {
		t.Errorf("DelegationID mismatch: got %q, want %q", restored.DelegationID, orig.DelegationID)
	}
	if restored.Type != orig.Type {
		t.Errorf("Type mismatch: got %q, want %q", restored.Type, orig.Type)
	}
	if restored.Content != orig.Content {
		t.Errorf("Content mismatch: got %q, want %q", restored.Content, orig.Content)
	}
	if len(restored.Options) != 3 {
		t.Errorf("Options length mismatch: got %d, want 3", len(restored.Options))
	}
	if restored.Resolution != orig.Resolution {
		t.Errorf("Resolution mismatch: got %q, want %q", restored.Resolution, orig.Resolution)
	}
	if !restored.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", restored.CreatedAt, now)
	}
	if restored.ResolvedAt == nil {
		t.Fatal("ResolvedAt should not be nil")
	}
	if !restored.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("ResolvedAt mismatch: got %v, want %v", restored.ResolvedAt, resolvedAt)
	}
}

func TestActivityEvent_JSON_OmitEmpty(t *testing.T) {
	// Activity with only required fields — optional fields should be omitted in JSON
	now := time.Now()
	activity := ActivityEvent{
		ID:           "act-min",
		DelegationID: "del-min",
		Type:         ActivityProgress,
		Content:      "Working on it",
		CreatedAt:    now,
		// Options, Resolution, ResolvedAt are zero values
	}

	data, err := json.Marshal(activity)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	// Required fields must be present
	for _, field := range []string{"id", "delegation_id", "type", "content", "created_at"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("required field %q missing from JSON", field)
		}
	}

	// Optional fields with zero values should be omitted
	for _, field := range []string{"options", "resolution", "resolved_at"} {
		if _, ok := raw[field]; ok {
			t.Errorf("optional field %q should be omitted when empty", field)
		}
	}
}

func TestActivityEvent_Options_OmitEmptySlice(t *testing.T) {
	// In Go's encoding/json, omitempty omits both nil AND empty slices.
	// This test verifies the behavior matches the struct tags.
	now := time.Now()

	// Nil options (should be omitted)
	withNilOptions := ActivityEvent{
		ID:           "act-1",
		DelegationID: "del-1",
		Type:         ActivityProgress,
		Content:      "test",
		CreatedAt:    now,
		Options:      nil,
	}
	data, _ := json.Marshal(withNilOptions)
	var rawNil map[string]interface{}
	_ = json.Unmarshal(data, &rawNil)
	if _, ok := rawNil["options"]; ok {
		t.Error("nil options should be omitted from JSON")
	}

	// Empty slice options — also omitted due to omitempty tag
	withEmptySlice := ActivityEvent{
		ID:           "act-2",
		DelegationID: "del-2",
		Type:         ActivityProgress,
		Content:      "test",
		CreatedAt:    now,
		Options:      []string{},
	}
	data, _ = json.Marshal(withEmptySlice)
	var rawSlice map[string]interface{}
	_ = json.Unmarshal(data, &rawSlice)
	if _, ok := rawSlice["options"]; ok {
		t.Error("empty slice options should be omitted from JSON due to omitempty tag")
	}
}
