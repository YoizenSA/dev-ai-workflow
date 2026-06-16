package engram

// ConsolidationPlan is the structured output the memory agent produces when
// asked to consolidate memories. The user reviews it and selectively applies
// items via the backend (never directly by the agent).
type ConsolidationPlan struct {
	Updates      []PlanUpdate  `json:"updates,omitempty"`
	Deletes      []PlanDelete  `json:"deletes,omitempty"`
	NewSummaries []PlanSummary `json:"new_summaries,omitempty"`
	Digest       string        `json:"digest,omitempty"`
}

// PlanUpdate proposes modifying an existing observation.
type PlanUpdate struct {
	ObservationID string  `json:"observation_id"`
	Reason        string  `json:"reason"`
	NewContent    string  `json:"new_content,omitempty"`
	NewScope      string  `json:"new_scope,omitempty"`
}

// PlanDelete proposes removing an observation (duplicate, obsolete).
type PlanDelete struct {
	ObservationID string `json:"observation_id"`
	Reason        string `json:"reason"`
}

// PlanSummary proposes creating a new summary/topic observation.
type PlanSummary struct {
	Type       string                 `json:"type"`
	Content    string                 `json:"content"`
	Scope      string                 `json:"scope"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
