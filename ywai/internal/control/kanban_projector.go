package control

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/kanban"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// KanbanProjector bridges missions events to the kanban board.
// It creates a kanban session per mission and a delegation per feature,
// updating their status as the mission progresses.
//
// Design: one-directional (missions→kanban). Moving a card in the kanban
// does NOT affect the mission FSM. Completed/cancelled missions leave
// their kanban session as history (no deletion).
type KanbanProjector struct {
	kanban   *kanban.Store
	missions *missions.MissionsStore
	mu       sync.Mutex
	sessions map[string]string            // missionID → kanban sessionID
	delegs   map[string]map[string]string // missionID → featureID → delegationID
}

// NewKanbanProjector creates a projector that pushes mission events to kanban.
func NewKanbanProjector(k *kanban.Store, m *missions.MissionsStore) *KanbanProjector {
	return &KanbanProjector{
		kanban:   k,
		missions: m,
		sessions: make(map[string]string),
		delegs:   make(map[string]map[string]string),
	}
}

// Project implements the mission event sink. It handles feature_status_changed
// events by creating/updating kanban sessions and delegations.
func (p *KanbanProjector) Project(evtType string, payload interface{}) {
	if evtType != "feature_status_changed" {
		return
	}

	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("KanbanProjector: invalid payload type %T", payload)
		return
	}

	missionID, _ := payloadMap["missionId"].(string)
	featureID, _ := payloadMap["featureId"].(string)

	// Bug fix: FeatureStatus is a named string type. Direct .(string) assertion
	// fails because the dynamic type is missions.FeatureStatus, not string.
	// We use fmt.Sprintf to normalize any type to string.
	var featureStatus string
	switch v := payloadMap["status"].(type) {
	case missions.FeatureStatus:
		featureStatus = string(v)
	case string:
		featureStatus = v
	default:
		featureStatus = fmt.Sprintf("%v", payloadMap["status"])
	}

	if missionID == "" || featureID == "" || featureStatus == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Ensure kanban session exists for this mission
	if _, ok := p.sessions[missionID]; !ok {
		if _, err := p.ensureSession(missionID); err != nil {
			log.Printf("KanbanProjector: %v", err)
			return
		}
	}

	// Update the delegation status
	delegID, ok := p.delegs[missionID][featureID]
	if !ok {
		log.Printf("KanbanProjector: unknown feature %s in mission %s", featureID, missionID)
		return
	}

	// Map FeatureStatus → MissionStatus before storing.
	// UpdateDelegation derives the kanban column from MissionStatus via
	// MapFSMToKanbanColumn. If we store raw FeatureStatus ("in_progress"),
	// it won't match MissionActive ("active") and falls to "backlog".
	// So we translate: in_progress → active, pending → pending, etc.
	missionStatus := mapFeatureStatusToMissionStatus(featureStatus)
	statusCopy := string(missionStatus)
	if _, err := p.kanban.UpdateDelegation(delegID, &statusCopy, nil, nil, nil, nil); err != nil {
		log.Printf("KanbanProjector: update delegation %s: %v", delegID, err)
	}
}

// ensureSession loads the mission and creates a kanban session + delegations.
// Must be called with p.mu held.
func (p *KanbanProjector) ensureSession(missionID string) (string, error) {
	mission, err := p.missions.LoadMission(missionID)
	if err != nil {
		return "", fmt.Errorf("load mission %s: %w", missionID, err)
	}

	project := mission.Project
	if project == "" {
		project = "missions"
	}

	session := p.kanban.CreateSession(project, mission.Name)
	sessionID := session.ID
	p.sessions[missionID] = sessionID
	p.delegs[missionID] = make(map[string]string)

	// Create a delegation per feature
	for _, f := range mission.Features {
		deps := f.Preconditions
		agent := normalizeAgent(f.SkillName)
		deleg, err := p.kanban.CreateDelegation(sessionID, agent, f.Description, deps)
		if err != nil {
			log.Printf("KanbanProjector: create delegation for feature %s: %v", f.ID, err)
			continue
		}
		p.delegs[missionID][f.ID] = deleg.ID
	}

	return sessionID, nil
}

// mapFeatureStatusToMissionStatus translates FeatureStatus → MissionStatus.
// This is the critical bridge: the engine emits FeatureStatus values
// ("pending", "in_progress", "completed", "failed", "cancelled"), but
// UpdateDelegation stores the status and derives the kanban column from
// MissionStatus via MapFSMToKanbanColumn. Without this translation,
// "in_progress" doesn't match MissionActive ("active") and falls to "backlog".
func mapFeatureStatusToMissionStatus(fs string) missions.MissionStatus {
	switch missions.FeatureStatus(fs) {
	case missions.FeaturePending:
		return missions.MissionPending
	case missions.FeatureInProgress:
		return missions.MissionActive
	case missions.FeatureCompleted:
		return missions.MissionCompleted
	case missions.FeatureFailed:
		return missions.MissionFailed
	case missions.FeatureCancelled:
		return missions.MissionCancelled
	default:
		return missions.MissionPending
	}
}

// normalizeAgent maps a worker-type name (e.g. "api", "ui", "tests") to a
// kanban agent category that the UI can color-code. Falls back to "dev".
func normalizeAgent(skillName string) string {
	// DesignWorkerSystem appends "-worker" to skill names (e.g. "api-worker").
	// Strip the suffix so the kanban color-coding works correctly.
	skillName = strings.TrimSuffix(skillName, "-worker")
	switch strings.ToLower(skillName) {
	case "api", "db", "infra", "implementation":
		return "dev"
	case "ui":
		return "dev"
	case "tests":
		return "qa"
	case "review":
		return "reviewer"
	case "architecture", "design":
		return "architect"
	case "devops", "deploy":
		return "devops"
	default:
		if skillName == "" {
			return "dev"
		}
		// Known kanban agents: dev, qa, reviewer, architect, devops
		// If the skillName already matches one, keep it
		known := map[string]bool{
			"dev": true, "qa": true, "reviewer": true,
			"architect": true, "devops": true,
		}
		if known[skillName] {
			return skillName
		}
		return "dev"
	}
}
