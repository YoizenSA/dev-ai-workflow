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
	stepMCP
	stepConfirm
	stepProgress
)

type agentOption struct {
	Name     string
	Binary   string
	Detected bool
}

type Model struct {
	step     step
	width    int
	height   int
	quitting bool

	agents        []agentOption
	agentCursor   int
	selectedAgent string

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
		step:   stepWelcome,
		agents: agentOpts,
	}
}

func (m *Model) advanceToNextValidStep() {
	if m.step == stepAgent && len(m.agents) == 0 {
		m.quitting = true
		return
	}
	// Skip MCP step if not opencode/kilocode
	// But ONLY after agent is selected (not when first entering Agent step)
	if m.step == stepAgent && m.selectedAgent != "" && !m.shouldShowMCPStep() {
		m.step = stepConfirm
	}
}

func (m *Model) shouldShowMCPStep() bool {
	if m.selectedAgent == "" {
		return false
	}
	return m.selectedAgent == "opencode" || m.selectedAgent == "kilocode"
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
	case stepMCP:
		m.step = stepAgent
	case stepConfirm:
		if m.shouldShowMCPStep() {
			m.step = stepMCP
		} else {
			m.step = stepAgent
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
		if m.shouldShowMCPStep() {
			m.step = stepMCP
		} else {
			m.step = stepConfirm
		}
		m.advanceToNextValidStep()
	case stepMCP:
		m.step = stepConfirm
	case stepConfirm:
		// Start installation and move to progress
		m.installAgent = m.selectedAgent
		m.step = stepProgress
		return m, m.startInstall()
	case stepProgress:
		// Wait for installation to complete
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
	case stepMCP:
		m.installMicrosoftLearnMCP = !m.installMicrosoftLearnMCP
	}
	return m, nil
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
	labels := []string{"Welcome", "Agent", "MCP", "Confirm", "Install"}
	steps := []step{stepWelcome, stepAgent, stepMCP, stepConfirm, stepProgress}

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
	b.WriteString(fmt.Sprintf("  Agent:         %s\n", agentLabel))

	b.WriteString(fmt.Sprintf("  Scope:         %s\n", dimStyle.Render("Global")))

	b.WriteString("\n  Skills to install: ")
	b.WriteString(skillStyle.Render("all extra skills"))
	b.WriteString("\n")

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
	return true
}

func Run(detectedAgents []agent.Agent) (string, bool, bool, error) {
	m := NewModel(detectedAgents)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return "", false, false, err
	}
	model := final.(*Model)
	return model.SelectedAgent(), model.InstallMicrosoftLearnMCP(), model.GlobalOnly(), nil
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
	// Find ywai binary in PATH
	ywaiBin, err := exec.LookPath("ywai")
	if err != nil {
		return "", fmt.Errorf("ywai not found in PATH: %w", err)
	}

	// Build the ywai command with selected flags
	args := []string{"install", "--agent", m.installAgent}

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
