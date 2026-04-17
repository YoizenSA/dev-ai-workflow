package main

import (
	"os"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer"
	tea "github.com/charmbracelet/bubbletea"
)

type installLogMsg struct {
	line string
}

type installFinishedMsg struct {
	err error
}

func (m setupModel) buildProjectInstallFlags() installer.Flags {
	flags := installer.Flags{}
	if m.baseFlags != nil {
		flags = *m.baseFlags
	}

	target := strings.TrimSpace(m.pathInput.Value())
	if target == "" {
		if wd, err := os.Getwd(); err == nil {
			target = wd
		}
	}

	flags.Target = target
	flags.ProjectType = m.projectTypeValues[m.projectTypeIdx]
	flags.Provider = m.providerValues[m.providerIdx]
	flags.UpdateAll = m.updateMode

	// componentValues indices MUST stay in sync with componentNames in
	// newSetupModel:
	//   0 GA, 1 SDD, 2 VSCode, 3 Extensions, 4 Global, 5 Hooks, 6 DryRun
	flags.InstallGA = m.componentValues[0]
	flags.InstallSDD = m.componentValues[1]
	flags.InstallVSCode = m.componentValues[2]
	flags.InstallExt = m.componentValues[3]
	flags.InstallGlobal = m.componentValues[4]
	flags.SkipHooks = !m.componentValues[5]
	if len(m.componentValues) > 6 {
		flags.DryRun = m.componentValues[6]
	}
	flags.Silent = true

	if strings.EqualFold(flags.Provider, "opencode") && !flags.SkipVSCode && !flags.InstallVSCode {
		flags.InstallVSCode = true
	}
	if strings.EqualFold(flags.Provider, "opencode") && !flags.InstallExt {
		flags.InstallExt = true
	}
	if strings.EqualFold(flags.Provider, "opencode") && !flags.InstallGlobal {
		flags.InstallGlobal = true
	}

	if !flags.UpdateAll && flags.InstallGA && flags.InstallSDD && flags.InstallVSCode && flags.InstallExt {
		flags.All = true
	}

	return flags
}

func (m setupModel) installPhaseTotal(flags installer.Flags) int {
	total := 0

	if !flags.SkipGA {
		total++ // Installing GA
	}
	if flags.InstallSDD && !flags.SkipSDD {
		total++ // Installing SDD
	}
	if flags.InstallVSCode && !flags.SkipVSCode {
		total++ // Installing VS Code extensions
	}

	// OpenCode, project configuration, and extensions are always part of the
	// main installation flow.
	total += 3 // OpenCode CLI + Configuring project + Installing extensions

	// Extra fine-grained stages that fire during extensions installation when
	// the relevant components are enabled. Tracking them individually keeps
	// the animated bar moving instead of jumping straight to done.
	if flags.InstallExt {
		total += 2 // hooks + install-steps
		if !flags.SkipHooks {
			total++ // additional hook-level signal
		}
	}
	if flags.InstallGlobal {
		total += 2 // global agents + engram
	}

	if total <= 0 {
		total = 1
	}

	return total
}

func (m setupModel) beginProjectInstallation() (tea.Model, tea.Cmd) {
	flags := m.buildProjectInstallFlags()

	if m.installStream == nil {
		m.installStream = &streamState{}
	}

	m.installLogs = nil
	m.installCurrent = ""
	m.installProgress = 0
	m.installTotal = m.installPhaseTotal(flags)
	m.installSeenStages = map[string]bool{}
	m.installErr = nil
	m.err = nil
	m.done = false
	m.step = stepInstalling

	return m, m.startInstallerCmd(flags)
}

func (m setupModel) startInstallerCmd(flags installer.Flags) tea.Cmd {
	stream := m.installStream

	return func() tea.Msg {
		if stream != nil {
			flags.Output = stream.writer
		}

		inst := installer.New(&flags)
		err := inst.Run()

		if err == nil {
			inst.ShowNextSteps()
		}

		if flusher, ok := flags.Output.(interface{ Flush() }); ok {
			flusher.Flush()
		}

		return installFinishedMsg{err: err}
	}
}

func (m setupModel) updateInstallLog(msg installLogMsg) (tea.Model, tea.Cmd) {
	line := strings.TrimRight(msg.line, "\r")
	if line == "" {
		return m, nil
	}

	m.installLogs = append(m.installLogs, line)
	if len(m.installLogs) > 18 {
		m.installLogs = append([]string(nil), m.installLogs[len(m.installLogs)-18:]...)
	}

	if stage := m.detectInstallStage(line); stage != "" {
		if m.installSeenStages == nil {
			m.installSeenStages = map[string]bool{}
		}
		if !m.installSeenStages[stage] {
			m.installSeenStages[stage] = true
			if m.installProgress < m.installTotal {
				m.installProgress++
			}
		}
		m.installCurrent = stage
	}

	return m, m.installBarSetPercent()
}

// installBarSetPercent returns the SetPercent command so the animated gradient
// bar eases toward the target fill ratio after each detected stage.
func (m setupModel) installBarSetPercent() tea.Cmd {
	if m.installTotal <= 0 {
		return nil
	}
	target := float64(m.installProgress) / float64(m.installTotal)
	if target < 0 {
		target = 0
	}
	if target > 1 {
		target = 1
	}
	return m.installBar.SetPercent(target)
}

func (m setupModel) updateInstallFinished(msg installFinishedMsg) (tea.Model, tea.Cmd) {
	m.installErr = msg.err
	m.done = true
	m.step = stepDone
	if msg.err == nil {
		m.installProgress = m.installTotal
	}
	return m, m.installBarSetPercent()
}

func (m setupModel) detectInstallStage(line string) string {
	clean := strings.ToLower(strings.TrimSpace(line))

	switch {
	case strings.Contains(clean, "ga already installed"),
		strings.Contains(clean, "installing ga"):
		return "Installing GA"
	case strings.Contains(clean, "sdd orchestrator installed"),
		strings.Contains(clean, "installing sdd"):
		return "Installing SDD"
	case strings.Contains(clean, "vs code cli not available"),
		strings.Contains(clean, "installing vs code extensions"):
		return "Installing VS Code extensions"
	case strings.Contains(clean, "opencode cli already installed"),
		strings.Contains(clean, "opencode cli installed"),
		strings.Contains(clean, "installing opencode cli"):
		return "Installing OpenCode CLI"
	case strings.Contains(clean, "configuring project"):
		return "Configuring project"
	case strings.Contains(clean, "installing hooks"),
		strings.Contains(clean, "installed ") && strings.Contains(clean, "hooks"):
		return "Installing hooks"
	case strings.Contains(clean, "installing mcps"),
		strings.Contains(clean, "installed ") && strings.Contains(clean, "mcps"):
		return "Installing MCP servers"
	case strings.Contains(clean, "installing install-steps"),
		strings.Contains(clean, "installed ") && strings.Contains(clean, "install-steps"):
		return "Installing install-steps"
	case strings.Contains(clean, "global agent"),
		strings.Contains(clean, "updated ") && strings.Contains(clean, "global agent"):
		return "Configuring global agents"
	case strings.Contains(clean, "engram"):
		return "Setting up Engram"
	case strings.Contains(clean, "context7"):
		return "Configuring Context7 MCP"
	case strings.Contains(clean, "installing extensions"):
		return "Installing extensions"
	default:
		return ""
	}
}
