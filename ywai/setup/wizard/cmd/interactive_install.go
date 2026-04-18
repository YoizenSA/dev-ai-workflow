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
	// newSetupModel. Custom mode maps each boolean to a granular flag.
	//   0 Docs (AGENTS.md / REVIEW.md)
	//   1 Skills
	//   2 Commands
	//   3 MCPs
	//   4 GA
	//   5 Engram
	//   6 Global agents
	//   7 Hooks
	//   8 Biome
	//   9 Plannotator
	//   10 Metronous
	//   11 SDD Engram Plugin
	//   12 Dry run
	if m.installModeIdx == 0 {
		// "All recommended" short-circuit: apply the happy-path defaults
		// regardless of whatever the user might have toggled in Custom
		// before bouncing back.
		flags.InstallGA = true
		flags.InstallSDD = true
		flags.InstallVSCode = true
		flags.InstallExt = true
		flags.InstallGlobal = true
		flags.SkipHooks = false
		flags.SkipBiome = true // opt-in, not part of "recommended"
	} else {
		flags.SkipDocs = !m.componentValues[0]
		flags.SkipSkills = !m.componentValues[1]
		flags.SkipCommands = !m.componentValues[2]
		flags.SkipMCPs = !m.componentValues[3]
		flags.InstallGA = m.componentValues[4]
		flags.SkipGA = !m.componentValues[4]
		flags.SkipEngram = !m.componentValues[5]
		flags.InstallGlobal = m.componentValues[6]
		flags.SkipHooks = !m.componentValues[7]
		flags.SkipBiome = !m.componentValues[8]
		if len(m.componentValues) > 9 {
			flags.InstallMetronous = m.componentValues[9]
		}
		if len(m.componentValues) > 10 {
			flags.SkipSddEngramPlugin = !m.componentValues[10]
		}
		if len(m.componentValues) > 11 {
			flags.DryRun = m.componentValues[11]
		}

		// Project integrations umbrella stays on in Custom mode too so the
		// hook / MCP / install-step pipeline runs (individual pieces are
		// gated by the Skip* flags).
		flags.InstallExt = true
		// SDD and VS Code default ON in custom too (they are not in the
		// 9-item list but remain part of the "integration" defaults).
		flags.InstallSDD = true
		flags.InstallVSCode = true
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
	m.installWarnings = nil
	m.installTail = nil
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

	// Keep a longer, non-trimmed tail that survives the transition to the
	// Done screen. This is what the user sees as "context" when something
	// flashed by too fast to read.
	m.installTail = append(m.installTail, line)
	if len(m.installTail) > 40 {
		m.installTail = append([]string(nil), m.installTail[len(m.installTail)-40:]...)
	}

	if isInstallWarningLine(line) {
		m.installWarnings = append(m.installWarnings, strings.TrimSpace(line))
		if len(m.installWarnings) > 10 {
			m.installWarnings = append([]string(nil), m.installWarnings[len(m.installWarnings)-10:]...)
		}
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

// isInstallWarningLine reports whether a log line from the installer looks
// like an error or a warning we should preserve and surface on the Done
// screen. We deliberately match conservatively to avoid flagging "no errors"
// lines or the generic "Installing ..." progress messages.
func isInstallWarningLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)

	// Fast negative: ignore common success/info phrasing even if it contains
	// the substring "error" (e.g., "no errors", "ignored error").
	if strings.HasPrefix(lower, "no errors") ||
		strings.Contains(lower, "without error") {
		return false
	}

	markers := []string{
		"error:",
		"error ",
		"failed",
		"failure",
		"fatal",
		"warning:",
		"warn:",
		"could not",
		"unable to",
		"refused",
		"denied",
		"missing",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
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
	// Clear the terminal on the Installing -> Done transition. The install
	// view is tall (progress bar, log box, status lines) and on several
	// terminals the shorter Done screen left leftover rows from the previous
	// frame around the success icon.
	return m, tea.Batch(m.installBarSetPercent(), tea.ClearScreen)
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
