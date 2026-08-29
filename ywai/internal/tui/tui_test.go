package tui

import (
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	tea "github.com/charmbracelet/bubbletea"
)

func testAgents() []agent.Agent {
	// Both must be profile-install hosts (windsurf is detected but not installable).
	return []agent.Agent{
		{Name: "opencode", BinaryName: "opencode"},
		{Name: "pi", BinaryName: "pi"},
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
	sendKey(m, "down")  // select custom (index 1)
	sendKey(m, "enter") // installMode -> agent
}

// Helper: navigate from welcome to quick install mode
func goToQuickInstall(m *Model) {
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
	m := NewModel(singleAgent("opencode"))
	if len(m.agents) != 1 {
		t.Fatalf("expected 1 agent option, got %d", len(m.agents))
	}
}

func TestNewModel_FiltersUnsupportedHosts(t *testing.T) {
	// windsurf/gemini may be on PATH but ywai has no profile install for them.
	m := NewModel([]agent.Agent{
		{Name: "opencode", BinaryName: "opencode"},
		{Name: "windsurf", BinaryName: "windsurf"},
		{Name: "gemini-cli", BinaryName: "gemini"},
		{Name: "omp", BinaryName: "omp"},
	})
	names := make([]string, 0, len(m.agents))
	for _, a := range m.agents {
		names = append(names, a.Name)
	}
	// opencode + omp + all
	if len(m.agents) != 3 {
		t.Fatalf("expected 3 options (2 supported + all), got %d: %v", len(m.agents), names)
	}
	for _, n := range names {
		if n == "windsurf" || n == "gemini-cli" {
			t.Fatalf("unsupported host %q must not appear in install list: %v", n, names)
		}
	}
}

func TestNewModel_Defaults(t *testing.T) {
	m := NewModel(testAgents())
	if m.step != stepInstallMode {
		t.Fatal("the wizard must open on the first real question, not a splash screen")
	}
	if strings.Contains(m.viewOptions(), "Scope") || strings.Contains(m.viewOptions(), "Global only") {
		t.Fatal("options must not expose retired scope controls")
	}
}

// The first keystroke must answer a question, not dismiss a splash screen.
func TestStepFlow_FirstEnterSelectsInstallMode(t *testing.T) {
	m := NewModel(testAgents())
	sendKey(&m, "enter")
	if m.step != stepAgent {
		t.Fatalf("expected stepAgent after the first enter, got %d", m.step)
	}
	if !m.quickInstall {
		t.Fatal("enter on the default row should have picked Quick Install")
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

func TestStepFlow_OptionsToMCP_WhenClaudeCode(t *testing.T) {
	m := NewModel(singleAgent("claude-code"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // select claude-code -> options
	sendKey(&m, "enter") // options -> optional plugins
	if m.step != stepMCP {
		t.Fatalf("expected stepMCP for claude-code, got %d", m.step)
	}
	if !m.shouldShowMCPStep() {
		t.Fatal("shouldShowMCPStep should be true for claude-code")
	}
}

// vscode-copilot has no MCP surface, so Options goes straight to Confirm.
func TestStepFlow_OptionsToConfirm_WhenNoMCPHost(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // select cursor -> options
	sendKey(&m, "enter") // options -> confirm (skip MCP)
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm for a host without MCP, got %d", m.step)
	}
}

func TestShouldShowMCPStep_All_WithOpencode(t *testing.T) {
	m := NewModel(testAgents()) // has opencode + pi
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
		{Name: "vscode-copilot", BinaryName: "code"},
		{Name: "pi", BinaryName: "pi"},
	}
	m := NewModel(agents)
	m.selectedAgent = "all"
	if m.shouldShowMCPStep() {
		t.Fatal("shouldShowMCPStep should be false when 'all' has no opencode/kilocode/claude")
	}
}

func TestOptionsStep_NavigationBounds(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	// Try going up from 0
	sendKey(&m, "up")
	if m.optionsCursor != 0 {
		t.Fatalf("cursor should stay at 0, got %d", m.optionsCursor)
	}
	// Go to bottom (Autostart). With no groups loaded the cursor
	// stops at the last option row instead of jumping to group selection.
	for i := 0; i < 10; i++ {
		sendKey(&m, "down")
	}
	if m.optionsCursor != optionsRowCount-1 {
		t.Fatalf("cursor should max at %d, got %d", optionsRowCount-1, m.optionsCursor)
	}
}

func TestEscNavigation(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	sendKey(&m, "enter") // options -> confirm (cursor skips MCP)
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
	sendKey(&m, "esc") // installMode is the first step: esc quits
	if !m.quitting {
		t.Fatal("esc on the first step should quit, not walk back to a splash screen")
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
	m := NewModel(singleAgent("vscode-copilot"))
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

func TestResult_AllFields(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
	m.selectedAgent = "vscode-copilot"
	m.installMicrosoftLearnMCP = true
	m.installPonytail = true
	m.confirmed = true

	r := m.Result()
	if r.Agent != "vscode-copilot" {
		t.Fatalf("Agent=%q, want vscode-copilot", r.Agent)
	}
	if !r.MCP {
		t.Fatal("MCP should be true")
	}
	if !r.Ponytail {
		t.Fatal("Ponytail should be true")
	}
}

func TestResult_PonytailOnByDefault(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
	m.selectedAgent = "vscode-copilot"
	m.confirmed = true
	r := m.Result()
	if !r.Ponytail {
		t.Fatalf("Ponytail must be on by default: %+v", r)
	}
	if r.MCP {
		t.Fatalf("MCP must stay off by default: %+v", r)
	}
}

// The Options step ends on Autostart: SDD is gone, so is the preset.
func TestOptionsStep_LastRowIsAutostart(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	for i := 0; i < optionsRowCount; i++ {
		sendKey(&m, "down")
	}
	if want := optionsRowCount - 1; m.optionsCursor != want {
		t.Fatalf("cursor must clamp at the last row %d, got %d", want, m.optionsCursor)
	}
	before := m.autostart
	sendKey(&m, "right")
	if m.autostart == before {
		t.Fatal("the last row should toggle autostart")
	}
}

func TestViewConfirm_ShowsAllOptions(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	m.selectedAgent = "opencode"
	m.installMicrosoftLearnMCP = true
	m.installPonytail = true

	view := m.viewConfirm()

	checks := []string{"opencode", "all extra skills", "Microsoft Learn MCP", "Ponytail"}
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
	m := NewModel(singleAgent("vscode-copilot"))
	m.step = stepOptions
	view := m.viewOptions()

	checks := []string{"Overwrite agents", "Autostart", "yes"}
	for _, c := range checks {
		if !strings.Contains(view, c) {
			t.Errorf("viewOptions missing %q", c)
		}
	}
	for _, gone := range []string{"Preset", "SDD", "Scope", "Global only"} {
		if strings.Contains(view, gone) {
			t.Errorf("viewOptions must not ask about %q any more", gone)
		}
	}
	if strings.Contains(view, "Persona") {
		t.Error("viewOptions must not offer gentle-ai Persona")
	}
}

func TestBreadcrumbs_IncludesOptions(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
	m.step = stepOptions
	bc := m.renderBreadcrumbs()
	if !strings.Contains(bc, "Options") {
		t.Fatal("breadcrumbs should include Options step")
	}
}

func TestBreadcrumbs_HidesOptionsInQuickMode(t *testing.T) {
	m := NewModel(singleAgent("vscode-copilot"))
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

func TestPonytailToggle(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	sendKey(&m, "enter") // options -> optional plugins
	if m.step != stepMCP {
		t.Fatalf("expected stepMCP, got %d", m.step)
	}
	if !m.installPonytail {
		t.Fatal("Ponytail should start as true")
	}
	if m.optionalPluginCursor != 0 {
		t.Fatalf("optionalPluginCursor=%d, want 0", m.optionalPluginCursor)
	}
	// Rows: 0 Microsoft Learn MCP, 1 Meta Developer Tools MCP, 2 Ponytail.
	sendKey(&m, "down")
	sendKey(&m, "down") // focus Ponytail
	if m.optionalPluginCursor != 2 {
		t.Fatalf("optionalPluginCursor=%d, want 2", m.optionalPluginCursor)
	}
	sendKey(&m, " ") // toggle ponytail off
	if m.installPonytail {
		t.Fatal("Ponytail should be false after space toggle")
	}
	if m.installMicrosoftLearnMCP {
		t.Fatal("MCP should remain false when toggling Ponytail")
	}
	if m.installMetaDevToolsMCP {
		t.Fatal("Meta MCP should remain false when toggling Ponytail")
	}
	sendKey(&m, " ") // toggle back on
	if !m.installPonytail {
		t.Fatal("Ponytail should be true after second toggle")
	}
}

func TestViewMCP_ShowsBothOptionalPlugins(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	m.selectedAgent = "opencode"
	m.step = stepMCP
	view := m.viewMCP()
	for _, want := range []string{"Microsoft Learn MCP", "Ponytail", "Lazy-senior"} {
		if !strings.Contains(view, want) {
			t.Errorf("viewMCP missing %q", want)
		}
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

// The Optional plugins rows and toggleCurrentMCP's switch are two lists that
// have to stay in the same order; a mismatch silently toggles the wrong
// plugin. This pins Meta Developer Tools to row 1.
func TestMetaDevToolsMCPToggle(t *testing.T) {
	m := NewModel(singleAgent("opencode"))
	goToCustomInstall(&m)
	sendKey(&m, "enter") // agent -> options
	sendKey(&m, "enter") // options -> optional plugins
	if m.step != stepMCP {
		t.Fatalf("expected stepMCP, got %d", m.step)
	}
	if m.optionalPluginCount() != 3 {
		t.Fatalf("optionalPluginCount()=%d, want 3", m.optionalPluginCount())
	}

	sendKey(&m, "down") // focus Meta Developer Tools MCP
	if m.optionalPluginCursor != 1 {
		t.Fatalf("optionalPluginCursor=%d, want 1", m.optionalPluginCursor)
	}
	before := m.installMetaDevToolsMCP
	sendKey(&m, " ")
	if m.installMetaDevToolsMCP == before {
		t.Fatal("space did not toggle Meta Developer Tools MCP")
	}
	if m.installMicrosoftLearnMCP {
		t.Fatal("Microsoft Learn MCP must not change when toggling Meta")
	}
	if !m.installPonytail {
		t.Fatal("Ponytail must not change when toggling Meta")
	}
}
