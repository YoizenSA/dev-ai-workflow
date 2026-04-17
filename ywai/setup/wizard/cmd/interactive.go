package main

import (
	"fmt"
	"os"
	"strings"

	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer"
	syncpkg "github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/sync"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type interactiveStep int

const (
	stepWelcome interactiveStep = iota
	stepPath
	stepProjectType
	stepProvider
	stepModel
	stepComponents
	stepConfirm
	stepSkillSelect
	stepSkillConfirm
	stepInstalling
	stepDone
	stepAgentType
	stepAgentName
	stepAgentDescription
	stepAgentPrompt
	stepAgentTools
	stepAgentConfirm
	stepAgentDone
	stepAgentList
	stepAgentMenu
	stepAgentView
	stepAgentEdit
	stepAgentDeleteConfirm
	stepFileBrowser
	stepGlobalTools
	stepGlobalToolsRunning
)

type setupModel struct {
	step       interactiveStep
	updateMode bool
	baseFlags  *installer.Flags

	width  int
	height int

	spinner       spinner.Model
	installBar    progress.Model
	globalToolBar progress.Model
	quitting      bool
	cancel        bool
	done          bool
	err           error

	pathInput textinput.Model

	projectTypeValues []string
	projectTypeLabels []string
	projectTypeHints  []string
	providerValues    []string
	providerLabels    []string

	modelInput      textinput.Model
	modelPresets    []string
	modelPresetIdx  int
	modelCustom     bool

	projectTypeIdx int
	providerIdx    int

	componentNames  []string
	componentValues []bool
	componentCursor int

	welcomeIdx     int
	welcomeOptions []string

	animationFrame int

	agentTypeIdx     int
	agentTypeOptions []string
	agentNameInput   textinput.Model
	agentDescInput   textinput.Model
	agentPromptInput textinput.Model
	agentToolNames   []string
	agentToolValues  []bool
	agentToolCursor  int
	agentCreated     bool
	agentError       error

	agentList        []AgentInfo
	agentListCursor  int
	agentToDelete    string
	agentSelected    string
	agentMenuCursor  int
	agentMenuOptions []string
	agentViewContent string
	agentEditField   int

	fileBrowserDir     string
	fileBrowserEntries []os.DirEntry
	fileBrowserCursor  int

	skillInstallMode bool
	skillOptions     []syncpkg.SkillInfo
	skillValues      []bool
	skillCursor      int
	skillLoadError   error

	installLogs       []string
	installCurrent    string
	installProgress   int
	installTotal      int
	installSeenStages map[string]bool
	installErr        error

	globalToolNames    []string
	globalToolValues   []bool
	globalToolCursor   int
	globalToolDone     bool
	globalToolOutput   string
	globalToolLogs     []string
	globalToolQueue    []int
	globalToolCurrent  string
	globalToolProgress int
	globalToolTotal    int
	globalToolStream   *streamState
	installStream      *streamState
}

func newSetupModel(defaultPath string, baseFlags *installer.Flags) setupModel {
	ti := textinput.New()
	ti.Placeholder = "~/my-project"
	ti.SetValue(defaultPath)
	ti.Focus()
	ti.Width = 50
	ti.Prompt = "  "

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	installBar := progress.New(
		progress.WithGradient("#00BFA5", "#5CE1E6"),
		progress.WithoutPercentage(),
		progress.WithWidth(36),
	)
	globalBar := progress.New(
		progress.WithGradient("#7D56F4", "#FF66C4"),
		progress.WithoutPercentage(),
		progress.WithWidth(36),
	)

	typeValues := []string{"generic", "nest", "nest-angular", "nest-react", "python", "dotnet", "devops"}
	typeLabels := []string{
		"generic      - Generic project",
		"nest         - NestJS backend",
		"nest-angular - NestJS + Angular",
		"nest-react   - NestJS + React",
		"python       - Python project",
		"dotnet       - .NET/C# project",
		"devops       - DevOps / infrastructure",
	}
	typeHints := []string{
		"Safe default if you are unsure",
		"Best for NestJS backends",
		"Best for NestJS + Angular repos",
		"Best for NestJS + React repos",
		"Best for Python apps and scripts",
		"Best for .NET / C# repos",
		"Best for CI/CD, Docker, Helm, k8s",
	}

	nameTi := textinput.New()
	nameTi.Placeholder = "my-agent"
	nameTi.Focus()
	nameTi.Width = 50
	nameTi.Prompt = "  "

	descTi := textinput.New()
	descTi.Placeholder = "Brief description of what this agent does"
	descTi.Width = 50
	descTi.Prompt = "  "

	promptTi := textinput.New()
	promptTi.Placeholder = "You are a specialized agent that..."
	promptTi.Width = 50
	promptTi.Prompt = "  "

	modelTi := textinput.New()
	modelTi.Placeholder = "anthropic/claude-sonnet-4"
	modelTi.Width = 50
	modelTi.Prompt = "  "

	return setupModel{
		step:      stepWelcome,
		baseFlags: baseFlags,
		welcomeOptions: []string{
			"Install YWAI in a project",
			"Update an existing YWAI setup",
			"Install missing skills in this repo",
			"Update global tools",
			"Create a global agent",
			"Manage global agents",
			"Quit",
		},
		welcomeIdx:        0,
		spinner:           s,
		installBar:        installBar,
		globalToolBar:     globalBar,
		pathInput:         ti,
		projectTypeValues: typeValues,
		projectTypeLabels: typeLabels,
		projectTypeHints:  typeHints,
		providerValues: []string{
			"opencode",
			"claude",
			"gemini",
			"ollama",
		},
		providerLabels: []string{
			"opencode - OpenCode + Copilot",
			"claude - Anthropic Claude",
			"gemini - Google Gemini",
			"ollama - Local Ollama",
		},
		modelInput: modelTi,
		modelPresets: []string{
			"(Use agent default)",
			"anthropic/claude-opus-4-20250514",
			"openai/codex-5.3",
			"anthropic/claude-sonnet-4-20250514",
			"google/gemini-3-flash",
			"google/gemini-3-1-pro",
			"anthropic/claude-haiku-4-5-20250514",
		},
		modelPresetIdx: 0,
		modelCustom:    false,
		componentNames: []string{
			"Core runtime: GA / base setup",
			"SDD Orchestrator",
			"VS Code + Copilot extensions",
			"Project integrations and extensions",
			"Global agents / skills",
			"Hooks (opencode-command-hooks, biome, etc.)",
			"Dry run (preview only)",
		},
		componentValues:    []bool{true, true, true, true, false, true, false},
		agentTypeIdx:       0,
		agentTypeOptions:   []string{"primary", "subagent"},
		agentNameInput:     nameTi,
		agentDescInput:     descTi,
		agentPromptInput:   promptTi,
		agentToolNames:     []string{"read", "write", "edit", "bash"},
		agentToolValues:    []bool{true, true, true, false},
		agentToolCursor:    0,
		agentCreated:       false,
		agentError:         nil,
		agentList:          []AgentInfo{},
		agentListCursor:    0,
		agentToDelete:      "",
		agentSelected:      "",
		agentMenuCursor:    0,
		agentMenuOptions:   []string{"View", "Edit", "Delete", "Back"},
		agentViewContent:   "",
		agentEditField:     0,
		fileBrowserDir:     "",
		fileBrowserEntries: []os.DirEntry{},
		fileBrowserCursor:  0,
		globalToolLogs:     []string{},
		globalToolStream:   &streamState{},
		installLogs:        []string{},
		installSeenStages:  map[string]bool{},
		installStream:      &streamState{},
	}
}

// animationTickMsg drives the header gradient cycle at a soft cadence.
type animationTickMsg time.Time

func animationTick() tea.Cmd {
	return tea.Tick(280*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg(t)
	})
}

// shouldAnimateHeader reports whether the header gradient tick is safe to
// schedule. During heavy I/O steps we stop re-ticking to avoid terminal
// ghosting on renderers that do not fully clear AltScreen between frames
// (seen on some Windows and Warp-like terminals). Motion in those steps is
// already provided by the spinner and the progress-bar easing.
func (m setupModel) shouldAnimateHeader() bool {
	switch m.step {
	case stepInstalling, stepGlobalToolsRunning, stepDone:
		return false
	}
	return true
}

func (m setupModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		animationTick(),
	)
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.step == stepInstalling {
			return m, nil
		}
		if m.step == stepDone {
			switch msg.String() {
			case "enter", "q", "esc", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			m.cancel = true
			m.quitting = true
			return m, tea.Quit
		}
	case installLogMsg:
		return m.updateInstallLog(msg)
	case installFinishedMsg:
		return m.updateInstallFinished(msg)
	case spinner.TickMsg:
		if m.step == stepDone {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case animationTickMsg:
		if !m.shouldAnimateHeader() {
			return m, nil
		}
		m.animationFrame++
		return m, animationTick()
	case progress.FrameMsg:
		var cmds []tea.Cmd
		var updated tea.Model
		updated, cmd := m.installBar.Update(msg)
		if bar, ok := updated.(progress.Model); ok {
			m.installBar = bar
		}
		cmds = append(cmds, cmd)
		updated, cmd = m.globalToolBar.Update(msg)
		if bar, ok := updated.(progress.Model); ok {
			m.globalToolBar = bar
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	switch m.step {
	case stepWelcome:
		return m.updateWelcome(msg)
	case stepPath:
		return m.updatePath(msg)
	case stepProjectType:
		return m.updateProjectType(msg)
	case stepProvider:
		return m.updateProvider(msg)
	case stepModel:
		return m.updateModel(msg)
	case stepComponents:
		return m.updateComponents(msg)
	case stepConfirm:
		return m.updateConfirm(msg)
	case stepSkillSelect:
		return m.updateSkillSelect(msg)
	case stepSkillConfirm:
		return m.updateSkillConfirm(msg)
	case stepAgentType:
		return m.updateAgentType(msg)
	case stepAgentName:
		return m.updateAgentName(msg)
	case stepAgentDescription:
		return m.updateAgentDescription(msg)
	case stepAgentPrompt:
		return m.updateAgentPrompt(msg)
	case stepAgentTools:
		return m.updateAgentTools(msg)
	case stepAgentConfirm:
		return m.updateAgentConfirm(msg)
	case stepAgentList:
		return m.updateAgentList(msg)
	case stepAgentMenu:
		return m.updateAgentMenu(msg)
	case stepAgentView:
		return m.updateAgentView(msg)
	case stepAgentEdit:
		return m.updateAgentEdit(msg)
	case stepAgentDeleteConfirm:
		return m.updateAgentDeleteConfirm(msg)
	case stepFileBrowser:
		return m.updateFileBrowser(msg)
	case stepGlobalTools:
		return m.updateGlobalTools(msg)
	case stepGlobalToolsRunning:
		return m.updateGlobalToolsRunning(msg)
	}

	return m, nil
}

func (m setupModel) View() string {
	if m.quitting {
		return m.renderQuitScreen()
	}

	if m.step == stepInstalling {
		return m.renderInstalling()
	}

	if m.step == stepDone {
		return m.renderDone()
	}

	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.step == stepAgentDone {
		content := m.renderAgentDone()
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center,
			lipgloss.Center,
			content,
		)
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	body := m.renderBody()

	mainContent := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		lipgloss.NewStyle().Height(1).Render(""),
		body,
		lipgloss.NewStyle().Height(1).Render(""),
		footer,
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center,
		lipgloss.Center,
		mainContent,
	)
}

func runInteractive(flags *installer.Flags) (bool, error) {
	wd, _ := os.Getwd()
	model := newSetupModel(wd, flags)

	var program *tea.Program
	program = tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)
	if model.globalToolStream != nil {
		model.globalToolStream.writer = newLineStreamWriter(func(line string) {
			program.Send(globalToolsLogMsg{line: line})
		})
	}
	if model.installStream != nil {
		model.installStream.writer = newLineStreamWriter(func(line string) {
			program.Send(installLogMsg{line: line})
		})
	}
	finalModel, err := program.Run()
	if err != nil {
		return false, err
	}

	m, ok := finalModel.(setupModel)
	if !ok {
		return false, fmt.Errorf("failed to read interactive state")
	}

	if m.cancel {
		return false, errInteractiveSetupCancelled
	}

	if !m.done {
		return false, errInteractiveSetupCancelled
	}

	if m.installErr != nil {
		return true, m.installErr
	}

	if !m.skillInstallMode {
		return true, nil
	}

	target := strings.TrimSpace(m.pathInput.Value())
	if target == "" {
		target = wd
	}

	flags.Target = target

	if m.skillInstallMode {
		var selected []string
		for idx, skill := range m.skillOptions {
			if idx < len(m.skillValues) && m.skillValues[idx] {
				selected = append(selected, skill.Name)
			}
		}
		if len(selected) == 0 {
			return false, fmt.Errorf("no skills selected")
		}
		flags.InstallSkills = selected
		return false, nil
	}

	flags.ProjectType = m.projectTypeValues[m.projectTypeIdx]
	flags.Provider = m.providerValues[m.providerIdx]
	flags.DefaultModel = m.getSelectedModel()
	flags.UpdateAll = m.updateMode

	flags.InstallGA = m.componentValues[0]
	flags.InstallSDD = m.componentValues[1]
	flags.InstallVSCode = m.componentValues[2]
	flags.InstallExt = m.componentValues[3]
	flags.InstallGlobal = m.componentValues[4]
	flags.SkipHooks = !m.componentValues[5]
	flags.DryRun = m.componentValues[6]

	if strings.EqualFold(flags.Provider, "opencode") && !flags.SkipVSCode && !flags.InstallVSCode {
		flags.InstallVSCode = true
		fmt.Println("ℹ OpenCode requires GitHub Copilot setup in this workflow; enabling VS Code extensions.")
	}
	if strings.EqualFold(flags.Provider, "opencode") && !flags.InstallExt {
		flags.InstallExt = true
		fmt.Println("ℹ OpenCode+Copilot flow requires project extensions; enabling extensions.")
	}
	if strings.EqualFold(flags.Provider, "opencode") && !flags.InstallGlobal {
		flags.InstallGlobal = true
		fmt.Println("ℹ OpenCode+Copilot flow requires global agents; enabling global skills/agents.")
	}

	if !flags.UpdateAll && flags.InstallGA && flags.InstallSDD && flags.InstallVSCode && flags.InstallExt {
		flags.All = true
	}

	return true, nil
}
