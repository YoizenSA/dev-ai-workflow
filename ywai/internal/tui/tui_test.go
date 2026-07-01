package tui

import (
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	tea "github.com/charmbracelet/bubbletea"
)

func testAgents() []agent.Agent {
	return []agent.Agent{
		{Name: "opencode", BinaryName: "opencode"},
		{Name: "windsurf", BinaryName: "windsurf"},
	}
}

func singleAgent(name string) []agent.Agent {
	return []agent.Agent{{Name: name, BinaryName: name}}
}

func sendKey(m *Model, key string) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	}
	m.Update(msg)
}

// Helper: navigate from welcome to custom install mode
func goToCustomInstall(m *Model) {
	sendKey(m, "enter") // welcome -> installMode
	sendKey(m, "down")  // select custom (index 1)
	sendKey(m, "enter") // installMode -> agent
}

// Helper: navigate from welcome to quick install mode
func goToQuickInstall(m *Model) {
	sendKey(m, "enter") // welcome -> installMode
	// quick is index 0 (default)
	sendKey(m, "enter") // installMode -> agent
}

func TestNewModel_MultipleAgentsHasAll(t *testing.T) {
	m := NewModel(testAgents())
	if len(m.agents) != 3 {
		t.Fatalf("expected 3 agent options (2 + all), got %d", len(m.agents))
	}
	if m.agents[2].Name != "all" {
		t.Fatalf("expected last option to be 'all', got %q", m.agents[2].Name)
	}
}

func TestNewModel_SingleAgentNoAll(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	if len(m.agents) != 1 {
		t.Fatalf("expected 1 agent option, got %d", len(m.agents))
	}
}

func TestNewModel_Defaults(t *testing.T) {
	m := NewModel(testAgents())
	if m.step != stepWelcome {
		t.Fatal("initial step should be stepWelcome")
	}
	if m.presetIdx != 0 || presetChoices[0] != "full-gentleman" {
		t.Fatal("default preset should be full-gentleman")
	}
	if m.globalOnly != true {
		t.Fatal("globalOnly should default to true")
	}
}

func TestStepFlow_WelcomeToInstallMode(t *testing.T) {
	m := NewModel(testAgents())
	sendKey(&m, "enter")
	if m.step != stepInstallMode {
		t.Fatalf("expected stepInstallMode after enter on welcome, got %d", m.step)
	}
}

func TestStepFlow_QuickInstallFlow(t *testing.T) {
	m := NewModel(testAgents())
	goToQuickInstall(&m)
	if m.step != stepAgent {
		t.Fatalf("expected stepAgent after quick install mode, got %d", m.step)
	}
	if !m.quickInstall {
		t.Fatal("quickInstall should be true")
	}
	// Select agent and go to confirm (skip options)
	sendKey(&m, "enter") // select opencode
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm for quick install, got %d", m.step)
	}
}

func TestStepFlow_CustomInstallFlow(t *testing.T) {
	m := NewModel(testAgents())
	goToCustomInstall(&m)
	if m.step != stepAgent {
		t.Fatalf("expected stepAgent after custom install mode, got %d", m.step)
	}
	if m.quickInstall {
		t.Fatal("quickInstall should be false")
	}
	// Select agent and go to options
	sendKey(&m, "enter") // select opencode
	if m.step != stepOptions {
		t.Fatalf("expected stepOptions for custom install, got %d", m.step)
	}
}

func TestStepFlow_OptionsToMCP_WhenOpencode(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // select opencode -> options
	sendKey(&m, "enter") // options -> MCP (because opencode)
	if m.step != stepMCP {
		t.Fatalf("expected stepMCP for opencode, got %d", m.step)
	}
}

func TestStepFlow_OptionsToConfirm_WhenWindsurf(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // select windsurf -> options
	sendKey(&m, "enter") // options -> confirm (skip MCP)
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm for windsurf (skip MCP), got %d", m.step)
	}
}

func TestShouldShowMCPStep_All_WithOpencode(t *testing.T) {
	m := NewModel(testAgents()) // has opencode + windsurf
	goToCustomInstall(&m)
	// Navigate to "all" (index 2)
	sendKey(&m, "down")
	sendKey(&m, "down")
	sendKey(&m, "enter") // select "all" -> options
	if m.selectedAgent != "all" {
		t.Fatalf("expected 'all', got %q", m.selectedAgent)
	}
	if !m.shouldShowMCPStep() {
		t.Fatal("shouldShowMCPStep should be true when 'all' is selected and opencode is among agents")
	}
}

func TestShouldShowMCPStep_All_NoOpencode(t *testing.T) {
	agents := []agent.Agent{
		{Name: "windsurf", BinaryName: "windsurf"},
		{Name: "cursor", BinaryName: "cursor"},
	}
	m := NewModel(agents)
	m.selectedAgent = "all"
	if m.shouldShowMCPStep() {
		t.Fatal("shouldShowMCPStep should be false when 'all' has no opencode/kilocode")
	}
}

func TestOptionsStep_CyclePreset(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	// Cursor starts at 0 (Preset)
	if m.presetIdx != 0 {
		t.Fatal("presetIdx should start at 0")
	}
	sendKey(&m, "right") // cycle preset forward
	if m.presetIdx != 1 {
		t.Fatalf("expected presetIdx=1 after right, got %d", m.presetIdx)
	}
	sendKey(&m, "right")
	if m.presetIdx != 2 {
		t.Fatalf("expected presetIdx=2, got %d", m.presetIdx)
	}
	sendKey(&m, "right") // wraps around
	if m.presetIdx != 0 {
		t.Fatalf("expected presetIdx=0 after wrap, got %d", m.presetIdx)
	}
	sendKey(&m, "left") // wraps backward
	if m.presetIdx != 2 {
		t.Fatalf("expected presetIdx=2 after left wrap, got %d", m.presetIdx)
	}
}

func TestOptionsStep_CycleGlobalOnly(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	// Navigate to Global only (row 2)
	sendKey(&m, "down") // -> Scope
	sendKey(&m, "down") // -> Global only
	if m.optionsCursor != 2 {
		t.Fatalf("expected optionsCursor=2, got %d", m.optionsCursor)
	}
	if !m.globalOnly {
		t.Fatal("globalOnly should be true initially")
	}
	sendKey(&m, "right")
	if m.globalOnly {
		t.Fatal("globalOnly should be false after toggle")
	}
	sendKey(&m, " ") // space also toggles
	if !m.globalOnly {
		t.Fatal("globalOnly should be true after second toggle")
	}
}

func TestOptionsStep_NavigationBounds(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	// Try going up from 0
	sendKey(&m, "up")
	if m.optionsCursor != 0 {
		t.Fatalf("cursor should stay at 0, got %d", m.optionsCursor)
	}
	// Go to bottom (5 = Overwrite agents). With no groups loaded the cursor
	// stops at the last option row instead of jumping to group selection.
	for i := 0; i < 10; i++ {
		sendKey(&m, "down")
	}
	if m.optionsCursor != optionsRowCount-1 {
		t.Fatalf("cursor should max at %d, got %d", optionsRowCount-1, m.optionsCursor)
	}
}

func TestEscNavigation(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	sendKey(&m, "enter") // options -> confirm (windsurf skips MCP)
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", m.step)
	}
	sendKey(&m, "esc") // confirm -> options (no MCP)
	if m.step != stepOptions {
		t.Fatalf("expected stepOptions on esc from confirm, got %d", m.step)
	}
	sendKey(&m, "esc") // options -> agent
	if m.step != stepAgent {
		t.Fatalf("expected stepAgent on esc from options, got %d", m.step)
	}
	sendKey(&m, "esc") // agent -> installMode
	if m.step != stepInstallMode {
		t.Fatalf("expected stepInstallMode on esc from agent, got %d", m.step)
	}
	sendKey(&m, "esc") // installMode -> welcome
	if m.step != stepWelcome {
		t.Fatalf("expected stepWelcome on esc from installMode, got %d", m.step)
	}
}

func TestEscNavigation_WithMCP(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	sendKey(&m, "enter") // options -> MCP
	if m.step != stepMCP {
		t.Fatalf("expected stepMCP, got %d", m.step)
	}
	sendKey(&m, "enter") // MCP -> confirm
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", m.step)
	}
	sendKey(&m, "esc") // confirm -> MCP
	if m.step != stepMCP {
		t.Fatalf("expected stepMCP on esc from confirm with opencode, got %d", m.step)
	}
	sendKey(&m, "esc") // MCP -> options
	if m.step != stepOptions {
		t.Fatalf("expected stepOptions on esc from MCP, got %d", m.step)
	}
}

func TestEscNavigation_QuickInstall(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	goToQuickInstall(&m)
	sendKey(&m, "enter") // agent -> confirm (quick)
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", m.step)
	}
	sendKey(&m, "esc") // confirm -> agent (quick)
	if m.step != stepAgent {
		t.Fatalf("expected stepAgent on esc from confirm in quick mode, got %d", m.step)
	}
}

func TestGlobalOnly_NotHardcoded(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	if !m.GlobalOnly() {
		t.Fatal("GlobalOnly should be true by default")
	}
	m.globalOnly = false
	if m.GlobalOnly() {
		t.Fatal("GlobalOnly should reflect model state")
	}
}

func TestResult_AllFields(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	m.selectedAgent = "windsurf"
	m.presetIdx = 1
	m.scopeIdx = 1
	m.globalOnly = true
	m.installMicrosoftLearnMCP = true
	m.confirmed = true

	r := m.Result()
	if r.Agent != "windsurf" {
		t.Fatalf("Agent=%q, want windsurf", r.Agent)
	}
	if r.Preset != "ecosystem-only" {
		t.Fatalf("Preset=%q, want ecosystem-only", r.Preset)
	}
	if r.Scope != "workspace" {
		t.Fatalf("Scope=%q, want workspace", r.Scope)
	}
	if !r.GlobalOnly {
		t.Fatal("GlobalOnly should be true")
	}
	if !r.MCP {
		t.Fatal("MCP should be true")
	}
}

func TestViewConfirm_ShowsAllOptions(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	m.selectedAgent = "opencode"
	m.presetIdx = 0
	m.scopeIdx = 0
	m.globalOnly = true
	m.installMicrosoftLearnMCP = true

	view := m.viewConfirm()

	checks := []string{"opencode", "full-gentleman", "global", "yes", "all extra skills", "Microsoft Learn MCP"}
	for _, c := range checks {
		if !strings.Contains(view, c) {
			t.Errorf("viewConfirm missing %q", c)
		}
	}
}

func TestViewConfirm_ShowsQuickInstallMode(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	m.selectedAgent = "opencode"
	m.quickInstall = true

	view := m.viewConfirm()
	if !strings.Contains(view, "Quick Install") {
		t.Error("viewConfirm should show 'Quick Install' when quickInstall is true")
	}
}

func TestViewOptions_Renders(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	m.step = stepOptions
	view := m.viewOptions()

	checks := []string{"Preset", "Scope", "Global only", "full-gentleman", "global", "yes"}
	for _, c := range checks {
		if !strings.Contains(view, c) {
			t.Errorf("viewOptions missing %q", c)
		}
	}
}

func TestBreadcrumbs_IncludesOptions(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	m.step = stepOptions
	bc := m.renderBreadcrumbs()
	if !strings.Contains(bc, "Options") {
		t.Fatal("breadcrumbs should include Options step")
	}
}

func TestBreadcrumbs_HidesOptionsInQuickMode(t *testing.T) {
	m := NewModel(singleAgent("windsurf"))
	m.quickInstall = true
	m.step = stepConfirm
	bc := m.renderBreadcrumbs()
	if strings.Contains(bc, "Options") {
		t.Fatal("breadcrumbs should hide Options in quick install mode")
	}
}

func TestMCPToggle(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	sendKey(&m, "enter") // options -> MCP
	if m.step != stepMCP {
		t.Fatalf("expected stepMCP, got %d", m.step)
	}
	if m.installMicrosoftLearnMCP {
		t.Fatal("MCP should start as false")
	}
	sendKey(&m, " ") // space to toggle
	if !m.installMicrosoftLearnMCP {
		t.Fatal("MCP should be true after space toggle")
	}
	sendKey(&m, " ") // space to toggle back
	if m.installMicrosoftLearnMCP {
		t.Fatal("MCP should be false after second toggle")
	}
}

func TestInstallMode_DefaultsToQuick(t *testing.T) {
	m := NewModel(testAgents())
	if m.installModeCursor != 0 {
		t.Fatal("installModeCursor should default to 0 (quick)")
	}
}

func TestViewInstallMode_Renders(t *testing.T) {
	m := NewModel(testAgents())
	m.step = stepInstallMode
	view := m.viewInstallMode()
	if !strings.Contains(view, "Quick Install") {
		t.Error("viewInstallMode should show 'Quick Install'")
	}
	if !strings.Contains(view, "Custom Install") {
		t.Error("viewInstallMode should show 'Custom Install'")
	}
}
