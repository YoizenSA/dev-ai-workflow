package control

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/kanban"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// ansiRe strips ANSI escape sequences from worker log lines so the kanban
// activity feed shows clean text.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// logMarkers are substrings that make a worker log line worth surfacing as a
// kanban activity — tool actions, shell commands, file writes, and results.
// Filtering keeps the live feed readable instead of dumping every raw line.
var logMarkers = []string{
	"$ ", "Write", "Read ", "Edit", "Glob", "Grep", "Bash", "Wrote",
	"go build", "go test", "Error", "error:", "FAIL", "PASS", " ok ",
	"✓", "✗", "Status:", "Did:", "Created", "Running", "Verify",
}

// KanbanProjector bridges missions events to the kanban board.
// It creates a kanban session per mission and a delegation per feature,
// updating their status as the mission progresses.
//
// Design: one-directional (missions→kanban). Moving a card in the kanban
// does NOT affect the mission FSM. Completed/cancelled missions leave
// their kanban session as history (no deletion).
type KanbanProjector struct {
	server      *kanban.Server
	kanban      *kanban.Store
	missions    *missions.MissionsStore
	mu          sync.Mutex
	sessions    map[string]string            // missionID → kanban sessionID
	delegs      map[string]map[string]string // missionID → featureID → delegationID
	onComplete  func(title, body string) error
}

// OnComplete sets a callback invoked when a delegation completes or fails.
func (p *KanbanProjector) OnComplete(fn func(title, body string) error) {
	p.onComplete = fn
}

// NewKanbanProjector creates a projector that pushes mission events to kanban.
// It takes the kanban Server (not just the Store) so it can broadcast live
// updates to connected UI clients the same way the HTTP handlers do.
func NewKanbanProjector(srv *kanban.Server, m *missions.MissionsStore) *KanbanProjector {
	return &KanbanProjector{
		server:   srv,
		kanban:   srv.Store(),
		missions: m,
		sessions: make(map[string]string),
		delegs:   make(map[string]map[string]string),
	}
}

// broadcast pushes a board update to UI clients when a server is wired.
func (p *KanbanProjector) broadcast(updateType string, payload interface{}) {
	if p.server != nil {
		p.server.Broadcast(updateType, payload)
	}
}

// addActivity appends an activity to a delegation and broadcasts it live.
func (p *KanbanProjector) addActivity(delegID, actType, content string) {
	act := &kanban.ActivityEvent{Type: kanban.ActivityType(actType), Content: content}
	if err := p.kanban.AddActivity(delegID, act); err != nil {
		return
	}
	p.broadcast("activity.created", act)
}

// Project implements the mission event sink. It handles feature_status_changed
// events by creating/updating kanban sessions and delegations.
func (p *KanbanProjector) Project(evtType string, payload interface{}) {
	switch evtType {
	case "feature_status_changed":
		p.handleFeatureStatus(payload)
	case "log_update":
		p.handleLogUpdate(payload)
	}
}

// handleLogUpdate surfaces meaningful worker log lines as live progress
// activities on the matching kanban card, so the user sees what each agent is
// actually doing in real time.
func (p *KanbanProjector) handleLogUpdate(payload interface{}) {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return
	}
	missionID, _ := m["missionId"].(string)
	featureID, _ := m["featureId"].(string)
	line, _ := m["line"].(string)
	if missionID == "" || featureID == "" {
		return
	}

	clean := strings.TrimSpace(ansiRe.ReplaceAllString(line, ""))
	if len(clean) < 3 || !isMeaningfulLog(clean) {
		return
	}
	if len(clean) > 200 {
		clean = clean[:200] + "…"
	}

	p.mu.Lock()
	delegID, ok := p.delegs[missionID][featureID]
	p.mu.Unlock()
	if !ok {
		return
	}
	p.addActivity(delegID, "progress", clean)
}

// isMeaningfulLog reports whether a cleaned log line is worth showing.
func isMeaningfulLog(line string) bool {
	for _, mk := range logMarkers {
		if strings.Contains(line, mk) {
			return true
		}
	}
	return false
}

func (p *KanbanProjector) handleFeatureStatus(payload interface{}) {
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
	d, err := p.kanban.UpdateDelegation(delegID, &statusCopy, nil, nil, nil, nil)
	if err != nil {
		log.Printf("KanbanProjector: update delegation %s: %v", delegID, err)
		return
	}
	// Broadcast the card move so the UI reflects it live.
	p.broadcast("delegation.status_changed", d)

	// Add a concise lifecycle activity so the timeline shows the agent moving.
	switch missions.FeatureStatus(featureStatus) {
	case missions.FeatureInProgress:
		p.addActivity(delegID, "progress", "▶ Agent started working")
	case missions.FeatureCompleted:
		p.addActivity(delegID, "progress", "✓ Completed")
		if p.onComplete != nil {
			_ = p.onComplete("ywai: Task Complete", "A delegation finished successfully")
		}
	case missions.FeatureFailed:
		p.addActivity(delegID, "blocked", "✗ Failed")
		if p.onComplete != nil {
			_ = p.onComplete("ywai: Task Failed", "A delegation encountered an error")
		}
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
	p.broadcast("session.created", session)

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
		p.broadcast("delegation.created", deleg)
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
