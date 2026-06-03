package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	primaryColor   = lipgloss.Color("99")
	secondaryColor = lipgloss.Color("86")
	tertiaryColor  = lipgloss.Color("208")
	successColor   = lipgloss.Color("84")
	errorColor     = lipgloss.Color("167")
	textSecondary  = lipgloss.Color("245")
	textMuted      = lipgloss.Color("241")
	borderColor    = lipgloss.Color("236")
	surfaceColor   = lipgloss.Color("235")
	textPrimary    = lipgloss.Color("255")

	bannerStyle   = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	titleStyle    = lipgloss.NewStyle().Foreground(primaryColor).Bold(true).MarginBottom(1)
	selStyle      = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Background(surfaceColor).Padding(0, 1)
	descStyle     = lipgloss.NewStyle().Foreground(textMuted)
	infoStyle     = lipgloss.NewStyle().Foreground(tertiaryColor)
	dimStyle      = lipgloss.NewStyle().Foreground(textMuted)
	skillStyle    = lipgloss.NewStyle().Foreground(tertiaryColor)
	okStyle       = lipgloss.NewStyle().Foreground(successColor)
	activeStyle   = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
	pendingStyle  = lipgloss.NewStyle().Foreground(textMuted)
	itemStyle     = lipgloss.NewStyle().Foreground(textPrimary)
	subtitleStyle = lipgloss.NewStyle().Foreground(textSecondary)
	monoStyle     = lipgloss.NewStyle().Foreground(secondaryColor)
	captionStyle  = lipgloss.NewStyle().Foreground(textMuted)
	warningStyle  = lipgloss.NewStyle().Foreground(tertiaryColor).Bold(true)
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

type step int

const (
	stepWelcome step = iota
	stepAgent
	stepOptions
	stepMCP
	stepConfirm
	stepProgress
)

var (
	presetChoices  = []string{"full-gentleman", "ecosystem-only", "minimal"}
	presetDescs    = map[string]string{
		"full-gentleman":  "Instala gentle-ai completo + todos los skills + agentes preconfigurados",
		"ecosystem-only":  "Solo gentle-ai core (sin skills extra)",
		"minimal":         "Solo lo esencial (gentle-ai básico)",
	}
	scopeChoices   = []string{"global", "workspace"}
	scopeDescs     = map[string]string{
		"global":     "Skills compartidos entre todos tus proyectos (~/.local)",
		"workspace":  "Skills solo en este proyecto (directorio actual)",
	}
	sddModeChoices = []string{"single", "multi"}
	sddModeDescs   = map[string]string{
		"single": "Un solo agente hace todo el trabajo",
		"multi":  "Divide trabajo entre múltiples agentes (build, plan, etc.)",
	}
	personaChoices = []string{"neutral", "gentleman"}
	personaDescs   = map[string]string{
		"neutral":   "Respuestas directas y concisas",
		"gentleman": "Más cortés y explicativo",
	}
)

type agentOption struct {
	Name     string
	Binary   string
	Detected bool
}

type TUIResult struct {
	Agent      string
	MCP        bool
	GlobalOnly bool
	Preset     string
	Scope      string
	SDDMode    string
	Persona    string
}

type Model struct {
	step     step
	width    int
	height   int
	quitting bool

	agents        []agentOption
	agentCursor   int
	selectedAgent string

	// Options step
	optionsCursor int
	presetIdx     int
	scopeIdx      int
	globalOnly    bool
	sddModeIdx    int
	personaIdx    int

	// MCP selection
	installMicrosoftLearnMCP bool

	// Progress state
	installOutput []string
	installDone   bool
	installError  error
	installAgent  string
}

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

	return Model{
		step:       stepWelcome,
		agents:     agentOpts,
		presetIdx:  0,
		scopeIdx:   0,
		globalOnly: true,
		sddModeIdx: 1,
		personaIdx: 0,
	}
}

func (m *Model) advanceToNextValidStep() {
	if m.step == stepAgent && len(m.agents) == 0 {
		m.quitting = true
		return
	}
}

func (m *Model) shouldShowMCPStep() bool {
	if m.selectedAgent == "" {
		return false
	}
	if m.selectedAgent == "opencode" || m.selectedAgent == "kilocode" {
		return true
	}
	if m.selectedAgent == "all" {
		for _, a := range m.agents {
			if a.Name == "opencode" || a.Name == "kilocode" {
				return true
			}
		}
	}
	return false
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case installTickMsg:
		// Just update spinner
		return m, nil

	case installDoneMsg:
		m.installDone = true
		m.installError = msg.err
		if msg.err != nil {
			m.installOutput = append(m.installOutput, "Installation failed!")
		} else {
			m.installOutput = append(m.installOutput, "Installation completed successfully!")
		}
		m.quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "esc":
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

func (m *Model) handleEsc() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		m.quitting = true
		return m, tea.Quit
	case stepAgent:
		m.step = stepWelcome
	case stepOptions:
		m.step = stepAgent
	case stepMCP:
		m.step = stepOptions
	case stepConfirm:
		if m.shouldShowMCPStep() {
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
		m.step = stepAgent
		m.advanceToNextValidStep()
	case stepAgent:
		if len(m.agents) == 0 {
			m.quitting = true
			return m, tea.Quit
		}
		m.selectedAgent = m.agents[m.agentCursor].Name
		m.step = stepOptions
	case stepOptions:
		if m.shouldShowMCPStep() {
			m.step = stepMCP
		} else {
			m.step = stepConfirm
		}
	case stepMCP:
		m.step = stepConfirm
	case stepConfirm:
		m.installAgent = m.selectedAgent
		m.step = stepProgress
		return m, m.startInstall()
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
	case stepAgent:
		if m.agentCursor > 0 {
			m.agentCursor--
		}
	case stepOptions:
		if m.optionsCursor > 0 {
			m.optionsCursor--
		}
	case stepMCP:
		m.installMicrosoftLearnMCP = !m.installMicrosoftLearnMCP
	}
	return m, nil
}

func (m *Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepAgent:
		if m.agentCursor < len(m.agents)-1 {
			m.agentCursor++
		}
	case stepOptions:
		if m.optionsCursor < 4 {
			m.optionsCursor++
		}
	case stepMCP:
		m.installMicrosoftLearnMCP = !m.installMicrosoftLearnMCP
	}
	return m, nil
}

func (m *Model) handleLeft() (tea.Model, tea.Cmd) {
	if m.step == stepOptions {
		m.cycleOption(-1)
	}
	return m, nil
}

func (m *Model) handleRight() (tea.Model, tea.Cmd) {
	if m.step == stepOptions {
		m.cycleOption(1)
	}
	return m, nil
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
		m.sddModeIdx = (m.sddModeIdx + dir + len(sddModeChoices)) % len(sddModeChoices)
	case 4:
		m.personaIdx = (m.personaIdx + dir + len(personaChoices)) % len(personaChoices)
	}
}

func (m *Model) View() string {
	if m.quitting && m.step != stepConfirm {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n")

	switch m.step {
	case stepWelcome:
		b.WriteString(m.viewWelcome())
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

	return b.String()
}

func (m *Model) renderBreadcrumbs() string {
	labels := []string{"Welcome", "Agent", "Options", "MCP", "Confirm", "Install"}
	steps := []step{stepWelcome, stepAgent, stepOptions, stepMCP, stepConfirm, stepProgress}

	var parts []string
	for i, label := range labels {
		if m.step == steps[i] {
			parts = append(parts, activeStyle.Render(fmt.Sprintf("● %s", label)))
		} else if m.step > steps[i] {
			parts = append(parts, okStyle.Render(fmt.Sprintf("✓ %s", label)))
		} else {
			parts = append(parts, pendingStyle.Render(fmt.Sprintf("○ %s", label)))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

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
	b.WriteString(renderLogo())
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("  Setup Wizard  •  AI Development Workflow"))
	b.WriteString("\n")
	b.WriteString(infoStyle.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")
	b.WriteString("  This will:\n")
	b.WriteString("    1. Install/update gentle-ai\n")
	b.WriteString("    2. Configure your AI agent with the Gentleman ecosystem\n")
	b.WriteString("    3. Copy all extra skills (React, Angular, TypeScript, etc.)\n")
	b.WriteString("\n")

	if len(m.agents) > 0 {
		detected := make([]string, 0, len(m.agents))
		for _, a := range m.agents {
			if a.Name != "all" {
				detected = append(detected, a.Name)
			}
		}
		b.WriteString(infoStyle.Render(fmt.Sprintf("  Detected agents: %s", strings.Join(detected, ", "))))
		b.WriteString("\n\n")
	}

	b.WriteString(dimStyle.Render("  Enter to start  •  q to quit"))
	return b.String()
}

func (m *Model) viewAgent() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select agent"))
	b.WriteString("\n\n")

	if len(m.agents) == 0 {
		b.WriteString(infoStyle.Render("  No agents detected."))
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
		cursor := " "
		if i == m.agentCursor {
			cursor = selStyle.Render("▶")
		}

		name := itemStyle.Render(a.Name)
		if i == m.agentCursor {
			name = selStyle.Render(a.Name)
		}

		pad := strings.Repeat(" ", maxNameLen-len(a.Name))

		if a.Name == "all" {
			desc := descStyle.Render("  Install for all detected agents")
			b.WriteString(fmt.Sprintf("  %s %s%s%s\n", cursor, name, pad, desc))
		} else {
			check := okStyle.Render("detected")
			pathInfo := descStyle.Render(fmt.Sprintf("  %s  (%s)", check, shortPath(a.Binary)))
			b.WriteString(fmt.Sprintf("  %s %s%s%s\n", cursor, name, pad, pathInfo))
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate  •  Enter select  •  Esc back"))
	return b.String()
}

func (m *Model) viewOptions() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Configure installation"))
	b.WriteString("\n\n")

	type optionRow struct {
		label string
		value string
	}

	globalLabel := "no"
	if m.globalOnly {
		globalLabel = "yes"
	}

	rows := []optionRow{
		{"Preset", presetChoices[m.presetIdx]},
		{"Scope", scopeChoices[m.scopeIdx]},
		{"Global only", globalLabel},
		{"SDD Mode", sddModeChoices[m.sddModeIdx]},
		{"Persona", personaChoices[m.personaIdx]},
	}

	maxLabel := 0
	for _, r := range rows {
		if len(r.label) > maxLabel {
			maxLabel = len(r.label)
		}
	}

	for i, r := range rows {
		cursor := "  "
		if i == m.optionsCursor {
			cursor = selStyle.Render("▶")
		}

		label := dimStyle.Render(r.label)
		if i == m.optionsCursor {
			label = itemStyle.Render(r.label)
		}

		pad := strings.Repeat(" ", maxLabel-len(r.label)+1)

		value := monoStyle.Render(r.value)
		if i == m.optionsCursor {
			value = selStyle.Render(r.value)
		}

		arrows := ""
		if i == m.optionsCursor {
			arrows = dimStyle.Render(" ◀ ▶")
		}

		b.WriteString(fmt.Sprintf("  %s %s%s%s%s\n", cursor, label, pad, value, arrows))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate  •  ←/→ change  •  Enter continue  •  Esc back"))

	// Show description of selected option
	b.WriteString("\n\n")
	switch m.optionsCursor {
	case 0: // Preset
		desc := presetDescs[presetChoices[m.presetIdx]]
		b.WriteString(dimStyle.Render("  " + desc))
	case 1: // Scope
		desc := scopeDescs[scopeChoices[m.scopeIdx]]
		b.WriteString(dimStyle.Render("  " + desc))
	case 2: // Global only
		if m.globalOnly {
			b.WriteString(dimStyle.Render("  Solo skills globales (sin AGENTS.md/REVIEW.md en el repo)"))
		} else {
			b.WriteString(dimStyle.Render("  Skills globales + AGENTS.md/REVIEW.md en el repo"))
		}
	case 3: // SDD Mode
		desc := sddModeDescs[sddModeChoices[m.sddModeIdx]]
		b.WriteString(dimStyle.Render("  " + desc))
	case 4: // Persona
		desc := personaDescs[personaChoices[m.personaIdx]]
		b.WriteString(dimStyle.Render("  " + desc))
	}

	return b.String()
}

func (m *Model) viewMCP() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Optional MCP servers"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Agent: %s\n\n", selStyle.Render(m.selectedAgent)))

	b.WriteString("  Select MCP servers to install:\n\n")

	cursor := "  "
	if m.installMicrosoftLearnMCP {
		cursor = selStyle.Render("[x]")
	} else {
		cursor = "[ ]"
	}

	name := itemStyle.Render("Microsoft Learn MCP")
	if m.installMicrosoftLearnMCP {
		name = selStyle.Render("Microsoft Learn MCP")
	}
	desc := descStyle.Render("  Acceso a documentación oficial de Microsoft")
	b.WriteString(fmt.Sprintf("  %s %s%s\n\n", cursor, name, desc))

	b.WriteString(okStyle.Render("  Enter to continue"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ toggle  •  Esc back"))
	return b.String()
}


func (m *Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Confirm installation"))
	b.WriteString("\n\n")

	agentLabel := m.selectedAgent
	if agentLabel == "" {
		agentLabel = warningStyle.Render("(none - BUG!)")
	} else if agentLabel == "all" {
		agentLabel = "all detected agents"
	}

	globalLabel := "no"
	if m.globalOnly {
		globalLabel = "yes"
	}

	rows := [][2]string{
		{"Agent", agentLabel},
		{"Preset", presetChoices[m.presetIdx]},
		{"Scope", scopeChoices[m.scopeIdx]},
		{"Global only", globalLabel},
		{"SDD Mode", sddModeChoices[m.sddModeIdx]},
		{"Persona", personaChoices[m.personaIdx]},
		{"Skills", "all extra skills"},
	}

	if m.shouldShowMCPStep() {
		mcpLabel := "no"
		if m.installMicrosoftLearnMCP {
			mcpLabel = "yes"
		}
		rows = append(rows, [2]string{"Microsoft Learn MCP", mcpLabel})
	}

	maxLabel := 0
	for _, r := range rows {
		if len(r[0]) > maxLabel {
			maxLabel = len(r[0])
		}
	}

	for _, r := range rows {
		pad := strings.Repeat(" ", maxLabel-len(r[0])+1)
		b.WriteString(fmt.Sprintf("  %s%s%s\n", dimStyle.Render(r[0]+":"), pad, monoStyle.Render(r[1])))
	}

	b.WriteString("\n")
	b.WriteString(okStyle.Render("  Enter to install"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Esc to go back"))
	return b.String()
}

func (m *Model) SelectedAgent() string {
	return m.selectedAgent
}

func (m *Model) InstallMicrosoftLearnMCP() bool {
	return m.installMicrosoftLearnMCP
}

func (m *Model) GlobalOnly() bool {
	return m.globalOnly
}

func (m *Model) Result() TUIResult {
	return TUIResult{
		Agent:      m.selectedAgent,
		MCP:        m.installMicrosoftLearnMCP,
		GlobalOnly: m.globalOnly,
		Preset:     presetChoices[m.presetIdx],
		Scope:      scopeChoices[m.scopeIdx],
		SDDMode:    sddModeChoices[m.sddModeIdx],
		Persona:    personaChoices[m.personaIdx],
	}
}

func Run(detectedAgents []agent.Agent) (TUIResult, error) {
	m := NewModel(detectedAgents)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return TUIResult{}, err
	}
	model := final.(*Model)
	return model.Result(), nil
}

func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// installProgressMsg is sent when installation output is received
type installProgressMsg string

// installDoneMsg is sent when installation completes
type installDoneMsg struct {
	err error
}

// installTickMsg is sent periodically to update the spinner
type installTickMsg time.Time

func (m *Model) startInstall() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return installTickMsg(t)
		}),
		func() tea.Msg {
			// Run installation in background
			m.installOutput = []string{"Starting installation..."}
			output, err := m.runInstall()
			if err != nil {
				return installDoneMsg{err: err}
			}
			// Split output into lines
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					m.installOutput = append(m.installOutput, line)
				}
			}
			return installDoneMsg{err: nil}
		},
	)
}

func (m *Model) runInstall() (string, error) {
	ywaiBin, err := exec.LookPath("ywai")
	if err != nil {
		return "", fmt.Errorf("ywai not found in PATH: %w", err)
	}

	args := []string{"install", "--agent", m.installAgent}
	args = append(args, "--preset", presetChoices[m.presetIdx])
	args = append(args, "--scope", scopeChoices[m.scopeIdx])
	args = append(args, "--sdd-mode", sddModeChoices[m.sddModeIdx])
	args = append(args, "--persona", personaChoices[m.personaIdx])
	if m.globalOnly {
		args = append(args, "--global")
	}
	if m.installMicrosoftLearnMCP {
		args = append(args, "--mcp")
	}

	cmd := exec.Command(ywaiBin, args...)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (m *Model) viewProgress() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Installing..."))
	b.WriteString("\n\n")

	// Show last 15 lines of output
	startLine := len(m.installOutput) - 15
	if startLine < 0 {
		startLine = 0
	}

	for i := startLine; i < len(m.installOutput); i++ {
		line := m.installOutput[i]
		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			// Highlight progress indicators
			b.WriteString(infoStyle.Render("  " + line))
		} else {
			b.WriteString("  " + itemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Show spinner at bottom
	if !m.installDone && m.installError == nil {
		spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spinIndex := int(time.Now().Unix()/100) % len(spinChars)
		b.WriteString("\n")
		b.WriteString(infoStyle.Render(fmt.Sprintf("  %s Installing...", spinChars[spinIndex])))
	} else if m.installError != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(errorColor).Render(fmt.Sprintf("  ✗ Installation failed: %v", m.installError)))
	} else {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(successColor).Bold(true).Render("  ✓ Installation complete!"))
	}

	b.WriteString("\n\n")

	return b.String()
}
