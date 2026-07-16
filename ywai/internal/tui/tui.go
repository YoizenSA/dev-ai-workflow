package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ──────────────────────────────────────────────────────────────────────────────
// Color palette
// ──────────────────────────────────────────────────────────────────────────────

var (
	primaryColor   = lipgloss.Color("99")
	secondaryColor = lipgloss.Color("86")
	tertiaryColor  = lipgloss.Color("208")
	successColor   = lipgloss.Color("84")
	errorColor     = lipgloss.Color("167")
	warningColor   = lipgloss.Color("214")
	textSecondary  = lipgloss.Color("245")
	textMuted      = lipgloss.Color("241")
	borderColor    = lipgloss.Color("236")
	textPrimary    = lipgloss.Color("255")
)

// ──────────────────────────────────────────────────────────────────────────────
// Styles
// ──────────────────────────────────────────────────────────────────────────────

var (
	titleStyle    = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).MarginBottom(1)
	descStyle     = lipgloss.NewStyle().Foreground(textMuted)
	infoStyle     = lipgloss.NewStyle().Foreground(tertiaryColor)
	dimStyle      = lipgloss.NewStyle().Foreground(textMuted)
	okStyle       = lipgloss.NewStyle().Foreground(successColor)
	activeStyle   = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
	pendingStyle  = lipgloss.NewStyle().Foreground(textMuted)
	itemStyle     = lipgloss.NewStyle().Foreground(textPrimary)
	subtitleStyle = lipgloss.NewStyle().Foreground(textSecondary)
	monoStyle     = lipgloss.NewStyle().Foreground(secondaryColor)
	helpStyle     = lipgloss.NewStyle().Foreground(textMuted).Italic(true)
	warningStyle  = lipgloss.NewStyle().Foreground(warningColor).Bold(true)
	checkStyle    = lipgloss.NewStyle().Foreground(successColor).Bold(true)
	crossStyle    = lipgloss.NewStyle().Foreground(errorColor).Bold(true)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2)

	// Section header
	sectionStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginTop(1).
			MarginBottom(0)

	// Progress bar
	progressEmptyStyle = lipgloss.NewStyle().Foreground(borderColor)
	progressFullStyle  = lipgloss.NewStyle().Foreground(successColor)

	// Step indicator for progress
	stepActiveStyle  = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
	stepDoneStyle    = lipgloss.NewStyle().Foreground(successColor)
	stepPendingStyle = lipgloss.NewStyle().Foreground(textMuted)
	stepErrorStyle   = lipgloss.NewStyle().Foreground(errorColor)
)

var brandPalette = []string{
	"99",
	"105",
	"111",
	"117",
	"86",
	"120",
	"150",
	"179",
	"183",
	"177",
}

var logoLines = []string{
	"██╗   ██╗██╗    ██╗ █████╗ ██╗",
	"╚██╗ ██╔╝██║    ██║██╔══██╗██║",
	" ╚████╔╝ ██║ █╗ ██║███████║██║",
	"  ╚██╔╝  ██║███╗██║██╔══██║██║",
	"   ██║   ╚███╔███╔╝██║  ██║██║",
	"   ╚═╝    ╚══╝╚══╝ ╚═╝  ╚═╝╚═╝",
}

// ──────────────────────────────────────────────────────────────────────────────
// Steps
// ──────────────────────────────────────────────────────────────────────────────

type step int

const (
	stepWelcome     step = iota
	stepInstallMode      // Quick vs Custom
	stepAgent
	stepOptions
	stepMCP
	stepConfirm
	stepProgress
)

// optionsRowCount is the number of navigable rows on the Options step
// (Preset, Scope, Global only, Overwrite agents, Autostart, SDD).
// SDD is optional (off by default). gentle-ai persona is never offered —
// ywai always owns agent tone via its curated AGENTS.md.
const optionsRowCount = 6

// ──────────────────────────────────────────────────────────────────────────────
// Choices
// ──────────────────────────────────────────────────────────────────────────────

var (
	presetChoices = []string{"full-gentleman", "ecosystem-only", "minimal"}
	// Presets only select gentle-ai components. ywai extra skills always install.
	presetDescs = map[string]string{
		"full-gentleman": "gentle-ai: engram + skills + context7 + permissions",
		"ecosystem-only": "gentle-ai: same core components as full (engram + skills + context7 + permissions)",
		"minimal":        "gentle-ai: skills component only (no engram via this preset)",
	}
	scopeChoices = []string{"global", "workspace"}
	scopeDescs   = map[string]string{
		"global":    "Skills shared across all your projects (~/.local)",
		"workspace": "Skills only in this project (current directory)",
	}
	// Optional gentle-ai SDD (index 0 = off). Persona is never installed.
	sddChoices = []string{"off", "single", "multi"}
	sddDescs   = map[string]string{
		"off":    "Do not install gentle-ai SDD (ywai skills still install)",
		"single": "gentle-ai SDD orchestrator in single-agent mode",
		"multi":  "gentle-ai SDD orchestrator in multi-agent mode",
	}
)

// ──────────────────────────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────────────────────────

type agentOption struct {
	Name     string
	Binary   string
	Detected bool
}

// TUIResult holds all configuration choices made in the TUI.
type TUIResult struct {
	Agent           string
	MCP             bool
	Ponytail        bool
	GlobalOnly      bool
	OverwriteAgents bool
	Preset          string
	Scope           string
	Autostart       bool
	// Optional gentle-ai SDD (false = skip). Persona is never installed.
	InstallSDD  bool
	SDDMode     string // single|multi when InstallSDD
	GroupFilter agents.GroupFilter
}

// InstallStep tracks a single installation phase.
type InstallStep struct {
	Name    string
	Status  string // "pending", "running", "done", "error"
	Message string
}

// Model is the Bubbletea model for the install TUI.
type Model struct {
	step      step
	width     int
	height    int
	quitting  bool
	confirmed bool // true only when user explicitly confirms at stepConfirm
	showHelp  bool // keymap overlay toggled with "?"

	// Install mode step
	installModeCursor int // 0 = quick, 1 = custom
	quickInstall      bool

	agents        []agentOption
	agentCursor   int
	selectedAgent string

	// Options step
	optionsCursor int
	presetIdx     int
	scopeIdx      int
	globalOnly    bool
	autostart     bool
	sddIdx        int // 0=off, 1=single, 2=multi

	// Optional plugins step (Microsoft Learn MCP + ponytail)
	optionalPluginCursor     int
	installMicrosoftLearnMCP bool
	installPonytail          bool

	// Overwrite existing profiles
	overwriteAgents bool

	// Agent groups selection
	availableGroups  []agents.GroupDefinition
	selectedGroups   map[string]bool
	groupNames       []string
	groupCursor      int
	showGroupOptions bool

	// Progress state
	installSteps []InstallStep
	installDone  bool
	installError error
	installAgent string
	spinnerFrame int
}

// ──────────────────────────────────────────────────────────────────────────────
// Constructor
// ──────────────────────────────────────────────────────────────────────────────

// NewModel creates a new TUI model with detected agents.
func NewModel(detectedAgents []agent.Agent) Model {
	hasAll := len(detectedAgents) > 1
	agentOpts := make([]agentOption, 0, len(detectedAgents))
	for _, a := range detectedAgents {
		agentOpts = append(agentOpts, agentOption{
			Name:     a.Name,
			Binary:   a.BinaryName,
			Detected: true,
		})
	}
	if hasAll {
		agentOpts = append(agentOpts, agentOption{
			Name:     "all",
			Binary:   "",
			Detected: true,
		})
	}

	// Load defaults from file or embedded
	defaults, err := config.LoadDefaults()
	if err != nil {
		defaults = config.BuiltInDefaults()
	}

	// Find indices for default values
	presetIdx := findIndex(presetChoices, defaults.Preset)
	scopeIdx := findIndex(scopeChoices, defaults.Scope)

	return Model{
		step:                     stepWelcome,
		agents:                   agentOpts,
		presetIdx:                presetIdx,
		scopeIdx:                 scopeIdx,
		globalOnly:               defaults.GlobalOnly,
		autostart:                defaults.Autostart,
		sddIdx:                   0, // SDD off by default
		installMicrosoftLearnMCP: defaults.MCP,
		installPonytail:          defaults.Ponytail,
		overwriteAgents:          true,
		selectedGroups:           make(map[string]bool),
		installSteps: []InstallStep{
			{Name: "Check gentle-ai", Status: "pending"},
			{Name: "Install ecosystem", Status: "pending"},
			{Name: "Copy extra skills", Status: "pending"},
			{Name: "Install agent profiles", Status: "pending"},
			{Name: "Apply overrides", Status: "pending"},
			{Name: "Install plugins", Status: "pending"},
		},
	}
}

// findIndex returns the index of value in choices, or 0 if not found.
func findIndex(choices []string, value string) int {
	for i, c := range choices {
		if c == value {
			return i
		}
	}
	return 0
}

// ──────────────────────────────────────────────────────────────────────────────
// Group helpers
// ──────────────────────────────────────────────────────────────────────────────

// LoadGroups loads the group manifest from the given source directory
// and populates the available/selected groups for the TUI.
func (m *Model) LoadGroups(sourceDir string) error {
	manifest, err := agents.LoadGroupManifest(sourceDir)
	if err != nil {
		return err
	}
	m.availableGroups = nil
	m.groupNames = nil
	m.selectedGroups = make(map[string]bool)
	// Core is always first
	if def, ok := manifest.Groups["core"]; ok {
		m.availableGroups = append(m.availableGroups, def)
		m.groupNames = append(m.groupNames, "core")
		m.selectedGroups["core"] = true
	}
	// All other groups in insertion order
	for name, def := range manifest.Groups {
		if name == "core" {
			continue
		}
		m.availableGroups = append(m.availableGroups, def)
		m.groupNames = append(m.groupNames, name)
		m.selectedGroups[name] = false
	}
	return nil
}

// GroupFilter builds a GroupFilter from the current selection.
func (m *Model) GroupFilter() agents.GroupFilter {
	var groups []string
	for name, selected := range m.selectedGroups {
		if selected && name != "core" {
			groups = append(groups, name)
		}
	}
	return agents.GroupFilter{Groups: groups}
}

// toggleGroup toggles the currently selected group (unless it's "core").
func (m *Model) toggleGroup() {
	if m.groupCursor >= 0 && m.groupCursor < len(m.groupNames) {
		name := m.groupNames[m.groupCursor]
		if name != "core" {
			m.selectedGroups[name] = !m.selectedGroups[name]
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Bubbletea lifecycle
// ──────────────────────────────────────────────────────────────────────────────

// Init initializes the Bubbletea model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and key presses.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// The help overlay swallows the next key: any key closes it.
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "?":
			m.showHelp = true
			return m, nil
		case "esc", "backspace":
			return m.handleEsc()
		case "enter":
			return m.handleEnter()
		case "up", "k":
			return m.handleUp()
		case "down", "j":
			return m.handleDown()
		case "left", "h":
			return m.handleLeft()
		case "right", "l", " ":
			return m.handleRight()
		default:
			// Any key exits when installation is done
			if m.step == stepProgress && (m.installDone || m.installError != nil) {
				m.quitting = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Key handlers
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) handleEsc() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		m.quitting = true
		return m, tea.Quit
	case stepInstallMode:
		m.step = stepWelcome
	case stepAgent:
		m.step = stepInstallMode
	case stepOptions:
		m.step = stepAgent
	case stepMCP:
		m.step = stepOptions
	case stepConfirm:
		if m.quickInstall {
			m.step = stepAgent
		} else if m.shouldShowMCPStep() {
			m.step = stepMCP
		} else {
			m.step = stepOptions
		}
	}
	return m, nil
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		m.step = stepInstallMode
	case stepInstallMode:
		if m.installModeCursor == 0 {
			// Quick install - go directly to agent selection, then install with defaults
			m.quickInstall = true
			m.step = stepAgent
		} else {
			// Custom install - go through all options
			m.quickInstall = false
			m.step = stepAgent
		}
	case stepAgent:
		if len(m.agents) == 0 {
			m.quitting = true
			return m, tea.Quit
		}
		m.selectedAgent = m.agents[m.agentCursor].Name
		if m.quickInstall {
			// Quick install - skip options, go to confirm
			m.step = stepConfirm
		} else {
			// Custom install - go to options
			m.step = stepOptions
		}
	case stepOptions:
		if m.showGroupOptions {
			m.toggleGroup()
		} else if m.shouldShowMCPStep() {
			m.step = stepMCP
		} else {
			m.step = stepConfirm
		}
	case stepMCP:
		m.step = stepConfirm
	case stepConfirm:
		m.installAgent = m.selectedAgent
		m.confirmed = true
		m.quitting = true
		return m, tea.Quit
	case stepProgress:
		if m.installDone || m.installError != nil {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) handleUp() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepInstallMode:
		m.installModeCursor = (m.installModeCursor + 1) % 2 // 2 items: wrap
	case stepAgent:
		if n := len(m.agents); n > 0 {
			m.agentCursor = (m.agentCursor - 1 + n) % n
		}
	case stepOptions:
		if m.showGroupOptions {
			if m.groupCursor > 0 {
				m.groupCursor--
			} else {
				// Move back to options rows
				m.showGroupOptions = false
				m.optionsCursor = optionsRowCount - 1
			}
		} else {
			if m.optionsCursor > 0 {
				m.optionsCursor--
			}
		}
	case stepMCP:
		n := m.optionalPluginCount()
		if n > 0 {
			m.optionalPluginCursor = (m.optionalPluginCursor - 1 + n) % n
		}
	}
	return m, nil
}

func (m *Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepInstallMode:
		m.installModeCursor = (m.installModeCursor + 1) % 2 // 2 items: wrap
	case stepAgent:
		if n := len(m.agents); n > 0 {
			m.agentCursor = (m.agentCursor + 1) % n
		}
	case stepOptions:
		if m.showGroupOptions {
			if m.groupCursor < len(m.groupNames)-1 {
				m.groupCursor++
			}
		} else {
			if m.optionsCursor < optionsRowCount-1 {
				m.optionsCursor++
			} else if len(m.groupNames) > 0 {
				// Move to group selection
				m.showGroupOptions = true
				m.groupCursor = 0
			}
		}
	case stepMCP:
		n := m.optionalPluginCount()
		if n > 0 {
			m.optionalPluginCursor = (m.optionalPluginCursor + 1) % n
		}
	}
	return m, nil
}

func (m *Model) handleLeft() (tea.Model, tea.Cmd) {
	if m.step == stepOptions {
		if m.showGroupOptions {
			m.toggleGroup()
		} else {
			m.cycleOption(-1)
		}
	}
	return m, nil
}

func (m *Model) handleRight() (tea.Model, tea.Cmd) {
	if m.step == stepOptions {
		if m.showGroupOptions {
			m.toggleGroup()
		} else {
			m.cycleOption(1)
		}
	} else if m.step == stepMCP {
		m.toggleCurrentMCP()
	}
	return m, nil
}

func (m *Model) toggleCurrentMCP() {
	if !m.shouldShowMCPStep() {
		return
	}
	// Row 0 = Microsoft Learn MCP, row 1 = Ponytail.
	switch m.optionalPluginCursor {
	case 0:
		m.installMicrosoftLearnMCP = !m.installMicrosoftLearnMCP
	case 1:
		m.installPonytail = !m.installPonytail
	}
}

// optionalPluginCount is the number of toggles on the Optional plugins step.
func (m *Model) optionalPluginCount() int {
	if m.shouldShowMCPStep() {
		return 2 // Microsoft Learn MCP + Ponytail
	}
	return 0
}

func (m *Model) cycleOption(dir int) {
	switch m.optionsCursor {
	case 0:
		m.presetIdx = (m.presetIdx + dir + len(presetChoices)) % len(presetChoices)
	case 1:
		m.scopeIdx = (m.scopeIdx + dir + len(scopeChoices)) % len(scopeChoices)
	case 2:
		m.globalOnly = !m.globalOnly
	case 3:
		m.overwriteAgents = !m.overwriteAgents
	case 4:
		m.autostart = !m.autostart
	case 5:
		m.sddIdx = (m.sddIdx + dir + len(sddChoices)) % len(sddChoices)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Conditional steps
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) shouldShowMCPStep() bool {
	if m.selectedAgent == "" {
		return false
	}
	// Optional plugins step: Microsoft Learn MCP + Ponytail for hosts we wire.
	// opencode/kilocode get both; claude-code gets both (MCP via settings.json,
	// ponytail via Claude marketplace CLI).
	if optionalPluginsAgent(m.selectedAgent) {
		return true
	}
	if m.selectedAgent == "all" {
		for _, a := range m.agents {
			if optionalPluginsAgent(a.Name) {
				return true
			}
		}
	}
	return false
}

func optionalPluginsAgent(name string) bool {
	switch name {
	case "opencode", "kilocode", "claude-code":
		return true
	default:
		return false
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// View
// ──────────────────────────────────────────────────────────────────────────────

// View renders the TUI.
func (m *Model) View() string {
	if m.quitting && m.step != stepConfirm && m.step != stepProgress {
		return ""
	}

	if m.showHelp {
		return m.viewHelpOverlay()
	}

	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n")

	switch m.step {
	case stepWelcome:
		b.WriteString(m.viewWelcome())
	case stepInstallMode:
		b.WriteString(m.viewInstallMode())
	case stepAgent:
		b.WriteString(m.viewAgent())
	case stepOptions:
		b.WriteString(m.viewOptions())
	case stepMCP:
		b.WriteString(m.viewMCP())
	case stepConfirm:
		b.WriteString(m.viewConfirm())
	case stepProgress:
		b.WriteString(m.viewProgress())
	}

	b.WriteString(m.renderFooter())

	return b.String()
}

// renderFooter draws a single, consistent key-hint line across the wizard
// steps so the available keys are always shown in the same place and order.
func (m *Model) renderFooter() string {
	var keys string
	switch m.step {
	case stepInstallMode, stepAgent:
		keys = "↑/↓ navigate  •  enter select"
	case stepOptions:
		if m.showGroupOptions {
			keys = "↑/↓ navigate  •  space toggle  •  enter continue"
		} else {
			keys = "↑/↓ navigate  •  ←/→ change  •  enter continue"
		}
	case stepMCP:
		keys = "↑/↓ navigate  •  space toggle  •  enter continue"
	default:
		return "" // welcome / confirm / progress have their own hints
	}
	global := "esc back  •  ? help  •  q quit"
	return "\n\n" + helpStyle.Render("  "+keys+"  •  "+global)
}

// viewHelpOverlay renders a full keymap, toggled with "?".
func (m *Model) viewHelpOverlay() string {
	rows := [][2]string{
		{"↑ / k", "Move up"},
		{"↓ / j", "Move down"},
		{"← / h", "Previous value"},
		{"→ / l", "Next value"},
		{"space", "Toggle option / checkbox"},
		{"enter", "Select / continue"},
		{"esc / backspace", "Go back"},
		{"?", "Toggle this help"},
		{"q / ctrl+c", "Quit"},
	}
	maxKey := 0
	for _, r := range rows {
		if w := lipgloss.Width(r[0]); w > maxKey {
			maxKey = w
		}
	}
	var lines []string
	for _, r := range rows {
		pad := strings.Repeat(" ", maxKey-lipgloss.Width(r[0])+3)
		lines = append(lines, monoStyle.Render(r[0])+pad+dimStyle.Render(r[1]))
	}
	box := boxStyle.Render(strings.Join(lines, "\n"))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(centerLine(titleStyle.Render("Keyboard shortcuts"), m.width))
	b.WriteString("\n\n")
	b.WriteString(centerBlock(box, m.width))
	b.WriteString("\n\n")
	b.WriteString(centerLine(helpStyle.Render("Press any key to close"), m.width))
	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Breadcrumbs
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) renderBreadcrumbs() string {
	labels := []string{"Welcome", "Mode", "Agent", "Options", "Plugins", "Confirm", "Install"}
	steps := []step{stepWelcome, stepInstallMode, stepAgent, stepOptions, stepMCP, stepConfirm, stepProgress}

	// Filter out steps based on mode
	var filteredLabels []string
	var filteredSteps []step
	for i, label := range labels {
		if steps[i] == stepMCP && !m.shouldShowMCPStep() {
			continue
		}
		if steps[i] == stepOptions && m.quickInstall {
			continue
		}
		filteredLabels = append(filteredLabels, label)
		filteredSteps = append(filteredSteps, steps[i])
	}

	var parts []string
	for i, label := range filteredLabels {
		if m.step == filteredSteps[i] {
			parts = append(parts, activeStyle.Render(fmt.Sprintf("● %s", label)))
		} else if m.step > filteredSteps[i] {
			parts = append(parts, checkStyle.Render(fmt.Sprintf("✓ %s", label)))
		} else {
			parts = append(parts, pendingStyle.Render(fmt.Sprintf("○ %s", label)))
		}
	}

	separator := dimStyle.Render("  ·  ")
	joined := strings.Join(parts, separator)
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, joined)
}

// ──────────────────────────────────────────────────────────────────────────────
// Welcome view
// ──────────────────────────────────────────────────────────────────────────────

func renderLogo() string {
	var out []string
	for i, line := range logoLines {
		if line == "" {
			out = append(out, line)
			continue
		}
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(brandPalette[i%len(brandPalette)]))
		out = append(out, style.Render(line))
	}
	return strings.Join(out, "\n")
}

func (m *Model) viewWelcome() string {
	var b strings.Builder

	// Logo - centered
	logo := renderLogo()
	b.WriteString(centerBlock(logo, m.width))
	b.WriteString("\n")

	// Subtitle - centered
	subtitle := subtitleStyle.Render("AI Development Workflow Setup")
	b.WriteString(centerLine(subtitle, m.width))
	b.WriteString("\n")

	// Divider - centered
	divider := infoStyle.Render(strings.Repeat("─", 36))
	b.WriteString(centerLine(divider, m.width))
	b.WriteString("\n\n")

	// Detected agents - centered
	if len(m.agents) > 0 {
		detected := make([]string, 0, len(m.agents))
		for _, a := range m.agents {
			if a.Name != "all" {
				detected = append(detected, a.Name)
			}
		}
		agentsLine := fmt.Sprintf("%s %s", dimStyle.Render("Agents:"), monoStyle.Render(strings.Join(detected, ", ")))
		b.WriteString(centerLine(agentsLine, m.width))
		b.WriteString("\n\n")
	}

	// Key hints - centered
	hints := dimStyle.Render("Enter to start  •  ? help  •  q to quit")
	b.WriteString(centerLine(hints, m.width))

	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Install mode view
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) viewInstallMode() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Installation mode"))
	b.WriteString("\n\n")

	modes := []struct {
		name string
		desc string
	}{
		{"Quick Install", "Use recommended defaults (just pick your agent)"},
		{"Custom Install", "Configure all options manually"},
	}

	for i, mode := range modes {
		isSelected := i == m.installModeCursor

		cursor := "  "
		if isSelected {
			cursor = activeStyle.Render(">")
		}

		name := itemStyle.Render(mode.name)
		if isSelected {
			name = activeStyle.Render(mode.name)
		}

		desc := descStyle.Render("  " + mode.desc)

		b.WriteString(fmt.Sprintf("  %s %s%s\n", cursor, name, desc))
	}

	// Quick install preview: show the defaults that will be applied so the user
	// isn't installing blind.
	if m.installModeCursor == 0 {
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("  Defaults that will be applied:"))
		b.WriteString("\n")
		defaults := [][2]string{
			{"Preset", presetChoices[m.presetIdx]},
			{"Scope", scopeChoices[m.scopeIdx]},
		}
		for _, d := range defaults {
			b.WriteString(fmt.Sprintf("    %s %s\n", dimStyle.Render(d[0]+":"), monoStyle.Render(d[1])))
		}
	}

	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Agent selection view
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) viewAgent() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select agent"))
	b.WriteString("\n\n")

	if len(m.agents) == 0 {
		b.WriteString(warningStyle.Render("  No agents detected."))
		b.WriteString("\n")
		return b.String()
	}

	maxNameLen := 0
	for _, a := range m.agents {
		if len(a.Name) > maxNameLen {
			maxNameLen = len(a.Name)
		}
	}

	for i, a := range m.agents {
		isSelected := i == m.agentCursor

		// Cursor
		cursor := "  "
		if isSelected {
			cursor = activeStyle.Render(">")
		}

		// Name
		name := itemStyle.Render(a.Name)
		if isSelected {
			name = activeStyle.Render(a.Name)
		}

		pad := strings.Repeat(" ", maxNameLen-len(a.Name)+2)

		if a.Name == "all" {
			desc := descStyle.Render("Install for all detected agents")
			b.WriteString(fmt.Sprintf("  %s %s%s%s\n", cursor, name, pad, desc))
		} else {
			check := checkStyle.Render("✓")
			pathInfo := descStyle.Render(fmt.Sprintf("%s  %s", check, shortPath(a.Binary)))
			b.WriteString(fmt.Sprintf("  %s %s%s%s\n", cursor, name, pad, pathInfo))
		}
	}

	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Options view
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) viewOptions() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Configure installation"))
	b.WriteString("\n")

	type optionRow struct {
		label string
		value string
	}

	globalLabel := "no"
	if m.globalOnly {
		globalLabel = "yes"
	}

	overwriteLabel := "no"
	if m.overwriteAgents {
		overwriteLabel = "yes"
	}

	autostartLabel := "no"
	if m.autostart {
		autostartLabel = "yes"
	}

	rows := []optionRow{
		{"Preset", presetChoices[m.presetIdx]},
		{"Scope", scopeChoices[m.scopeIdx]},
		{"Global only", globalLabel},
		{"Overwrite agents", overwriteLabel},
		{"Autostart", autostartLabel},
		{"SDD", sddChoices[m.sddIdx]},
	}

	maxLabel := 0
	for _, r := range rows {
		if len(r.label) > maxLabel {
			maxLabel = len(r.label)
		}
	}

	for i, r := range rows {
		isSelected := i == m.optionsCursor && !m.showGroupOptions

		// Cursor
		cursor := "  "
		if isSelected {
			cursor = activeStyle.Render(">")
		}

		// Label
		label := dimStyle.Render(r.label)
		if isSelected {
			label = itemStyle.Render(r.label)
		}

		pad := strings.Repeat(" ", maxLabel-len(r.label)+1)

		// Value
		value := renderValue(r.value)
		if isSelected {
			value = activeStyle.Render(r.value)
		}

		// Arrows
		arrows := ""
		if isSelected {
			arrows = dimStyle.Render(" ◀ ▶")
		}

		b.WriteString(fmt.Sprintf("  %s %s%s%s%s\n", cursor, label, pad, value, arrows))
	}

	// Description of selected option
	b.WriteString("\n")
	if !m.showGroupOptions {
		switch m.optionsCursor {
		case 0:
			b.WriteString(helpStyle.Render("    " + presetDescs[presetChoices[m.presetIdx]]))
		case 1:
			b.WriteString(helpStyle.Render("    " + scopeDescs[scopeChoices[m.scopeIdx]]))
		case 2:
			if m.globalOnly {
				b.WriteString(helpStyle.Render("    Global skills only (no AGENTS.md/REVIEW.md in the repo)"))
			} else {
				b.WriteString(helpStyle.Render("    Global skills + AGENTS.md/REVIEW.md in the repo"))
			}
		case 3:
			if m.overwriteAgents {
				b.WriteString(helpStyle.Render("    Overwrite existing agent profiles with fresh copies"))
			} else {
				b.WriteString(helpStyle.Render("    Keep existing agent profiles untouched"))
			}
		case 4:
			if m.autostart {
				b.WriteString(helpStyle.Render("    Start the control server on boot (auto-updates first, then serves)"))
			} else {
				b.WriteString(helpStyle.Render("    Don't configure the control server to start on boot"))
			}
		case 5:
			b.WriteString(helpStyle.Render("    " + sddDescs[sddChoices[m.sddIdx]]))
		}
	}

	// Agent groups section
	if len(m.groupNames) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("Agent Groups"))
		b.WriteString("\n")

		for i, name := range m.groupNames {
			isGroupSelected := m.showGroupOptions && i == m.groupCursor

			gCursor := "  "
			if isGroupSelected {
				gCursor = activeStyle.Render(">")
			}

			checked := dimStyle.Render("[ ]")
			if m.selectedGroups[name] {
				checked = checkStyle.Render("[✓]")
			}

			gName := itemStyle.Render(name)
			if isGroupSelected {
				gName = activeStyle.Render(name)
			}

			gDesc := ""
			if i < len(m.availableGroups) {
				gDesc = m.availableGroups[i].Description
			}

			if name == "core" {
				desc := descStyle.Render("  " + gDesc + " (always installed)")
				b.WriteString(fmt.Sprintf("  %s %s %s %s\n", gCursor, checked, gName, desc))
			} else {
				desc := descStyle.Render("  " + gDesc)
				b.WriteString(fmt.Sprintf("  %s %s %s%s\n", gCursor, checked, gName, desc))
			}
		}
	}

	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// MCP view
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) viewMCP() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Optional plugins"))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("  Agent: %s\n\n", activeStyle.Render(m.selectedAgent)))

	b.WriteString("  Select plugins to install:\n\n")

	if !m.shouldShowMCPStep() {
		return b.String()
	}

	type pluginRow struct {
		name    string
		desc    string
		checked bool
	}
	rows := []pluginRow{
		{
			name:    "Microsoft Learn MCP",
			desc:    "Access to official Microsoft documentation",
			checked: m.installMicrosoftLearnMCP,
		},
		{
			name:    "Ponytail",
			desc:    "Lazy-senior mode: YAGNI / stdlib (OpenCode plugin or Claude marketplace)",
			checked: m.installPonytail,
		},
	}

	for i, row := range rows {
		isSelected := i == m.optionalPluginCursor

		cursor := "  "
		if isSelected {
			cursor = activeStyle.Render(">")
		}

		checkbox := dimStyle.Render("[ ]")
		if row.checked {
			checkbox = checkStyle.Render("[✓]")
		}

		name := itemStyle.Render(row.name)
		if isSelected {
			name = activeStyle.Render(row.name)
		}
		desc := descStyle.Render("  " + row.desc)
		b.WriteString(fmt.Sprintf("  %s %s %s%s\n", cursor, checkbox, name, desc))
	}

	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Confirm view
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) viewConfirm() string {
	var b strings.Builder

	// Centered title
	title := titleStyle.Render("Confirm installation")
	b.WriteString(centerLine(title, m.width))
	b.WriteString("\n")

	// Mode indicator
	modeText := "Custom Install"
	if m.quickInstall {
		modeText = "Quick Install"
	}
	mode := infoStyle.Render("Mode: " + modeText)
	b.WriteString(centerLine(mode, m.width))
	b.WriteString("\n\n")

	agentLabel := m.selectedAgent
	if agentLabel == "" {
		agentLabel = warningStyle.Render("(none)")
	} else if agentLabel == "all" {
		agentLabel = "all detected agents"
	}

	globalLabel := "no"
	if m.globalOnly {
		globalLabel = "yes"
	}

	overwriteLabel := "no"
	if m.overwriteAgents {
		overwriteLabel = "yes"
	}

	autostartLabel := "no"
	if m.autostart {
		autostartLabel = "yes"
	}

	rows := [][2]string{
		{"Agent", agentLabel},
		{"Preset", presetChoices[m.presetIdx]},
		{"Scope", scopeChoices[m.scopeIdx]},
		{"Global only", globalLabel},
		{"Overwrite agents", overwriteLabel},
		{"Autostart", autostartLabel},
		{"SDD", sddChoices[m.sddIdx]},
		{"Skills", "all extra skills (always)"},
	}

	// Add selected groups
	if len(m.selectedGroups) > 0 {
		var groupNames []string
		for name, selected := range m.selectedGroups {
			if selected && name != "core" {
				groupNames = append(groupNames, name)
			}
		}
		if len(groupNames) > 0 {
			sort.Strings(groupNames)
			rows = append(rows, [2]string{"Groups", strings.Join(groupNames, ", ")})
		}
	}

	if m.shouldShowMCPStep() {
		mcpLabel := "no"
		if m.installMicrosoftLearnMCP {
			mcpLabel = "yes"
		}
		rows = append(rows, [2]string{"Microsoft Learn MCP", mcpLabel})
		ponytailLabel := "no"
		if m.installPonytail {
			ponytailLabel = "yes"
		}
		rows = append(rows, [2]string{"Ponytail", ponytailLabel})
	}

	// Find max label width for alignment
	maxLabel := 0
	for _, r := range rows {
		if len(r[0]) > maxLabel {
			maxLabel = len(r[0])
		}
	}

	// Render rows with consistent spacing
	var lines []string
	for _, r := range rows {
		label := dimStyle.Render(r[0])
		pad := strings.Repeat(" ", maxLabel-len(r[0])+3)
		value := renderValue(r[1])
		lines = append(lines, label+pad+value)
	}

	// Box with inner padding
	box := boxStyle.Render(strings.Join(lines, "\n"))
	b.WriteString(centerBlock(box, m.width))
	b.WriteString("\n\n")

	// Centered action hints
	hint := okStyle.Render("Enter to install") + dimStyle.Render("  •  Esc to go back  •  ? help")
	b.WriteString(centerLine(hint, m.width))

	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Progress view
// ──────────────────────────────────────────────────────────────────────────────

func (m *Model) viewProgress() string {
	var b strings.Builder

	if m.installDone && m.installError == nil {
		b.WriteString(checkStyle.Render("  ✓ Installation complete!"))
	} else if m.installError != nil {
		b.WriteString(crossStyle.Render("  ✗ Installation failed"))
	} else {
		b.WriteString(titleStyle.Render("  Installing..."))
	}
	b.WriteString("\n\n")

	// Spinner
	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := ""
	if !m.installDone && m.installError == nil {
		spinner = spinnerChars[m.spinnerFrame]
	}

	// Steps
	for i, step := range m.installSteps {
		icon := ""
		nameStyle := stepPendingStyle
		msgStyle := dimStyle

		switch step.Status {
		case "pending":
			icon = pendingStyle.Render("○")
			nameStyle = stepPendingStyle
		case "running":
			icon = stepActiveStyle.Render(spinner)
			nameStyle = stepActiveStyle
			msgStyle = infoStyle
		case "done":
			icon = stepDoneStyle.Render("✓")
			nameStyle = stepDoneStyle
		case "error":
			icon = stepErrorStyle.Render("✗")
			nameStyle = stepErrorStyle
			msgStyle = stepErrorStyle
		}

		name := nameStyle.Render(step.Name)
		msg := ""
		if step.Message != "" {
			msg = " " + msgStyle.Render(step.Message)
		}

		b.WriteString(fmt.Sprintf("    %s %s%s\n", icon, name, msg))

		// Show progress bar for running step
		if step.Status == "running" && i < len(m.installSteps)-1 {
			progress := float64(i) / float64(len(m.installSteps))
			barWidth := 30
			filled := int(progress * float64(barWidth))
			bar := progressFullStyle.Render(strings.Repeat("█", filled)) +
				progressEmptyStyle.Render(strings.Repeat("░", barWidth-filled))
			b.WriteString(fmt.Sprintf("      %s %d%%\n", bar, int(progress*100)))
		}
	}

	b.WriteString("\n")

	if m.installDone && m.installError == nil {
		b.WriteString("\n")
		b.WriteString(okStyle.Render("  Press any key to exit"))
	} else if m.installError != nil {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to exit"))
	}

	return b.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Public accessors
// ──────────────────────────────────────────────────────────────────────────────

// SelectedAgent returns the selected agent name.
func (m *Model) SelectedAgent() string {
	return m.selectedAgent
}

// InstallMicrosoftLearnMCP returns whether MCP should be installed.
func (m *Model) InstallMicrosoftLearnMCP() bool {
	return m.installMicrosoftLearnMCP
}

// GlobalOnly returns whether global-only mode is enabled.
func (m *Model) GlobalOnly() bool {
	return m.globalOnly
}

// Result returns the TUIResult with all configuration choices.
// Returns empty Agent if the user did not explicitly confirm.
func (m *Model) Result() TUIResult {
	if !m.confirmed {
		return TUIResult{}
	}
	sddChoice := sddChoices[m.sddIdx]
	r := TUIResult{
		Agent:           m.selectedAgent,
		MCP:             m.installMicrosoftLearnMCP,
		Ponytail:        m.installPonytail,
		GlobalOnly:      m.globalOnly,
		OverwriteAgents: m.overwriteAgents,
		Preset:          presetChoices[m.presetIdx],
		Scope:           scopeChoices[m.scopeIdx],
		Autostart:       m.autostart,
		GroupFilter:     m.GroupFilter(),
	}
	if sddChoice != "off" {
		r.InstallSDD = true
		r.SDDMode = sddChoice
	}
	return r
}

// ──────────────────────────────────────────────────────────────────────────────
// Run
// ──────────────────────────────────────────────────────────────────────────────

// Run starts the TUI and returns the result.
func Run(detectedAgents []agent.Agent) (TUIResult, error) {
	m := NewModel(detectedAgents)
	// Try source dir first (has latest groups when running from source checkout),
	// fall back to data dir (seeded/embedded).
	if err := m.LoadGroups(config.AgentsSourceDir()); err != nil {
		if err2 := m.LoadGroups(config.DataAgentsDir()); err2 != nil {
			// Non-fatal: groups.json might not exist, TUI still works without groups
		}
	}
	p := tea.NewProgram(&m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return TUIResult{}, err
	}
	model := final.(*Model)
	return model.Result(), nil
}

// centerLine centers a styled string within the given width.
// lipgloss styles add ANSI codes that break len(), so we strip them for width calc.
func centerLine(s string, width int) string {
	if width <= 0 {
		return "  " + s
	}
	visible := lipgloss.Width(s)
	pad := (width - visible) / 2
	if pad < 1 {
		pad = 1
	}
	return strings.Repeat(" ", pad) + s
}

// centerBlock centers a multi-line block (e.g. a box) within the given width.
func centerBlock(block string, width int) string {
	if width <= 0 {
		return block
	}
	blockWidth := lipgloss.Width(block)
	pad := (width - blockWidth) / 2
	if pad < 1 {
		pad = 1
	}
	indent := strings.Repeat(" ", pad)
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		lines = append(lines, indent+line)
	}
	return strings.Join(lines, "\n")
}

// renderValue styles a value cell: green for "yes", dim for "no", mono otherwise.
func renderValue(v string) string {
	switch v {
	case "yes":
		return okStyle.Render("yes")
	case "no":
		return dimStyle.Render("no")
	default:
		return monoStyle.Render(v)
	}
}

func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// ──────────────────────────────────────────────────────────────────────────────
// Messages (install logic removed — commands.go handles execution)
// ──────────────────────────────────────────────────────────────────────────────

// Install logic removed from TUI — commands.go handles execution.
// The TUI only collects configuration; it does not run install itself.

// ──────────────────────────────────────────────────────────────────────────────
// Messages (install logic removed — commands.go handles execution)
// ──────────────────────────────────────────────────────────────────────────────

// Install logic removed from TUI — commands.go handles execution.
// The TUI only collects configuration; it does not run install itself.
