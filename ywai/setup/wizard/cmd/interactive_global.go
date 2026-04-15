package main

import (
	"fmt"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Global tools

type globalToolsStartMsg struct{}

type globalToolsStepDoneMsg struct {
	output string
}

type globalToolsLogMsg struct {
	line string
}

func (m setupModel) updateGlobalTools(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.globalToolCursor > 0 {
				m.globalToolCursor--
			}
		case "down", "j":
			if m.globalToolCursor < len(m.globalToolNames)-1 {
				m.globalToolCursor++
			}
		case " ":
			m.globalToolValues[m.globalToolCursor] = !m.globalToolValues[m.globalToolCursor]
		case "a":
			allSelected := true
			for _, v := range m.globalToolValues {
				if !v {
					allSelected = false
					break
				}
			}
			for i := range m.globalToolValues {
				m.globalToolValues[i] = !allSelected
			}
		case "enter":
			m.globalToolOutput = ""
			m.globalToolLogs = nil
			m.globalToolDone = false
			m.globalToolQueue = m.globalToolQueue[:0]
			m.globalToolCurrent = ""
			m.globalToolProgress = 0

			for idx, selected := range m.globalToolValues {
				if selected {
					m.globalToolQueue = append(m.globalToolQueue, idx)
				}
			}

			m.globalToolTotal = len(m.globalToolQueue)
			if m.globalToolTotal == 0 {
				m.globalToolOutput = "No global tools selected."
				m.globalToolDone = true
				m.step = stepGlobalTools
				return m, nil
			}

			m.step = stepGlobalToolsRunning
			return m, func() tea.Msg {
				return globalToolsStartMsg{}
			}
		case "q", "esc":
			m.step = stepWelcome
		}
	}
	return m, nil
}

func (m setupModel) updateGlobalToolsRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case globalToolsLogMsg:
		line := strings.TrimRight(msg.line, "\r")
		if line == "" && len(m.globalToolLogs) == 0 {
			return m, nil
		}

		m.globalToolLogs = append(m.globalToolLogs, line)
		if len(m.globalToolLogs) > 16 {
			m.globalToolLogs = append([]string(nil), m.globalToolLogs[len(m.globalToolLogs)-16:]...)
		}
		return m, nil

	case globalToolsStartMsg:
		m.spinner, _ = m.spinner.Update(spinner.TickMsg{})

		if len(m.globalToolQueue) == 0 {
			m.globalToolDone = true
			m.step = stepGlobalTools
			if strings.TrimSpace(m.globalToolOutput) == "" {
				m.globalToolOutput = "Done."
			}
			return m, nil
		}

		nextIndex := m.globalToolQueue[0]
		m.globalToolQueue = m.globalToolQueue[1:]
		label := m.globalToolNames[nextIndex]
		m.globalToolCurrent = label

		return m, func() tea.Msg {
			var buf strings.Builder
			// Keep Bubble Tea in control of the screen; installer logs are
			// routed into a live stream below instead of writing directly to the
			// terminal and corrupting the TUI.
			flags := &installer.Flags{
				Force:      true,
				Silent:     true,
				DryRun:     false,
				Channel:    installer.DEFAULT_CHANNEL,
				GlobalOnly: true, // Don't write to repo during global tools update
			}

			if m.globalToolStream != nil {
				flags.Output = m.globalToolStream.writer
			}

			inst := installer.New(flags)
			buf.WriteString(fmt.Sprintf("  %s ...\n", label))

			var err error
			switch nextIndex {
			case 0: // GA
				err = inst.UpdateGA()
			case 1: // SDD
				err = inst.UpdateSDD()
			case 2: // Global agents
				err = inst.UpdateGlobalAgents()
			case 3: // Engram
				err = inst.UpdateEngram()
			case 4: // Context7
				err = inst.UpdateContext7()
			default:
				err = fmt.Errorf("unknown global tool index: %d", nextIndex)
			}

			if flusher, ok := flags.Output.(interface{ Flush() }); ok {
				flusher.Flush()
			}

			if err != nil {
				buf.WriteString(fmt.Sprintf("    ERROR: %v\n", err))
			} else {
				buf.WriteString("    OK\n")
			}

			return globalToolsStepDoneMsg{
				output: buf.String(),
			}
		}

	case globalToolsStepDoneMsg:
		if strings.TrimSpace(msg.output) != "" {
			if m.globalToolOutput != "" && !strings.HasSuffix(m.globalToolOutput, "\n") {
				m.globalToolOutput += "\n"
			}
			m.globalToolOutput += msg.output
		}

		m.globalToolProgress++
		m.globalToolCurrent = ""

		if len(m.globalToolQueue) == 0 {
			m.globalToolDone = true
			m.step = stepGlobalTools
			if strings.TrimSpace(m.globalToolOutput) == "" {
				m.globalToolOutput = "Done."
			} else {
				if !strings.HasSuffix(m.globalToolOutput, "\n") {
					m.globalToolOutput += "\n"
				}
				m.globalToolOutput += "Done."
			}
			return m, nil
		}

		return m, func() tea.Msg {
			return globalToolsStartMsg{}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m setupModel) renderGlobalToolsStep() string {
	subtitle := subtitleStyle.Render("Select which global tools to update (no repo needed)")

	var items []string
	for idx, name := range m.globalToolNames {
		marker := "[ ]"
		if m.globalToolValues[idx] {
			marker = "[x]"
		}
		line := fmt.Sprintf("%s %s", marker, name)
		if idx == m.globalToolCursor {
			items = append(items, selectedItemStyle.Render("> "+line))
		} else {
			items = append(items, itemStyle.Render("  "+line))
		}
	}

	menu := lipgloss.JoinVertical(lipgloss.Left, items...)

	parts := []string{
		subtitle,
		"",
		infoStyle.Render("Space toggle | a select all | Enter confirm | q back"),
		"",
		menu,
	}

	if m.globalToolOutput != "" {
		maxW := 50
		if m.width > 0 {
			maxW = m.width / 2
			if maxW < 40 {
				maxW = 40
			}
		}
		box := boxStyle.Width(maxW).Render(m.globalToolOutput)
		parts = append(parts, "", box)
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m setupModel) renderGlobalToolsRunningStep() string {
	parts := []string{
		subtitleStyle.Render("Updating global tools..."),
		"",
	}

	if m.globalToolTotal > 0 {
		parts = append(parts,
			renderProgressBar(m.globalToolProgress, m.globalToolTotal, m.globalToolsProgressWidth()),
			"",
			infoStyle.Render(fmt.Sprintf("%d/%d complete", m.globalToolProgress, m.globalToolTotal)),
			"",
		)
	}

	parts = append(parts,
		m.spinner.View()+" Working...",
	)

	if m.globalToolCurrent != "" {
		parts = append(parts,
			"",
			titleStyle.Render("Now updating: "+m.globalToolCurrent),
		)
	}

	if len(m.globalToolLogs) > 0 {
		maxW := 70
		if m.width > 0 {
			maxW = m.width / 2
			if maxW < 40 {
				maxW = 40
			}
		}
		parts = append(parts,
			"",
			boxStyle.Width(maxW).Render(strings.Join(m.globalToolLogs, "\n")),
		)
	}

	parts = append(parts,
		"",
		helpStyle.Render("Each tool updates in sequence, so the bar reflects real completed work."),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		parts...,
	)
}
