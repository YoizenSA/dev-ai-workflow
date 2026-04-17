package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m setupModel) renderHeader() string {
	logoLines := []string{
		"██╗   ██╗██╗    ██╗ █████╗ ██╗",
		"╚██╗ ██╔╝██║    ██║██╔══██╗██║",
		" ╚████╔╝ ██║ █╗ ██║███████║██║",
		"  ╚██╔╝  ██║███╗██║██╔══██║██║",
		"   ██║   ╚███╔███╔╝██║  ██║██║",
		"   ╚═╝    ╚══╝╚══╝ ╚═╝  ╚═╝╚═╝",
	}

	styledLogo := m.renderAnimatedLogo(logoLines)
	version := subtitleStyle.Render("Setup Wizard  •  AI Development Workflow")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		styledLogo,
		"",
		version,
		"",
		infoStyle.Render(strings.Repeat("─", 50)),
	)
}

func (m setupModel) renderBody() string {
	if m.step == stepWelcome {
		return m.renderWelcomeStep()
	}

	// Only the install / skill-install wizard has a linear step sequence;
	// global tools and agent-management flows are standalone and should not
	// display the Path/Type/... indicator (which caused confusion).
	showStepIndicator := !m.isStandaloneFlow()
	stepIndicator := ""
	if showStepIndicator {
		stepIndicator = m.renderStepIndicator()
	}

	content := ""
	switch m.step {
	case stepPath:
		content = m.renderPathStep()
	case stepProjectType:
		content = m.renderProjectTypeStep()
	case stepProvider:
		content = m.renderProviderStep()
	case stepModel:
		content = m.renderModelStep()
	case stepComponents:
		content = m.renderComponentsStep()
	case stepConfirm:
		content = m.renderConfirmStep()
	case stepSkillSelect:
		content = m.renderSkillSelectStep()
	case stepSkillConfirm:
		content = m.renderSkillConfirmStep()
	case stepAgentType:
		content = m.renderAgentTypeStep()
	case stepAgentName:
		content = m.renderAgentNameStep()
	case stepAgentDescription:
		content = m.renderAgentDescriptionStep()
	case stepAgentPrompt:
		content = m.renderAgentPromptStep()
	case stepAgentTools:
		content = m.renderAgentToolsStep()
	case stepAgentConfirm:
		content = m.renderAgentConfirmStep()
	case stepAgentList:
		content = m.renderAgentListStep()
	case stepAgentMenu:
		content = m.renderAgentMenuStep()
	case stepAgentView:
		content = m.renderAgentViewStep()
	case stepAgentEdit:
		content = m.renderAgentEditStep()
	case stepAgentDeleteConfirm:
		content = m.renderAgentDeleteConfirmStep()
	case stepFileBrowser:
		content = m.renderFileBrowserStep()
	case stepGlobalTools:
		content = m.renderGlobalToolsStep()
	case stepGlobalToolsRunning:
		content = m.renderGlobalToolsRunningStep()
	}

	if !showStepIndicator {
		return content
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		stepIndicator,
		lipgloss.NewStyle().Height(1).Render(""),
		content,
	)
}

// isStandaloneFlow reports whether the current step belongs to a flow that
// does not share the Path/Type/... wizard sequence.
func (m setupModel) isStandaloneFlow() bool {
	switch m.step {
	case stepGlobalTools,
		stepGlobalToolsRunning,
		stepAgentType,
		stepAgentName,
		stepAgentDescription,
		stepAgentPrompt,
		stepAgentTools,
		stepAgentConfirm,
		stepAgentList,
		stepAgentMenu,
		stepAgentView,
		stepAgentEdit,
		stepAgentDeleteConfirm,
		stepFileBrowser:
		return true
	}
	return false
}

func (m setupModel) renderStepIndicator() string {
	if m.skillInstallMode {
		stepNames := []string{"Path", "Skills", "Review"}
		stepEnums := []interactiveStep{stepPath, stepSkillSelect, stepSkillConfirm}
		var parts []string
		for i, s := range stepNames {
			idx := stepEnums[i]
			var item string
			switch {
			case idx < m.step:
				item = successStyle.Render("● " + s)
			case idx == m.step:
				item = titleStyle.Render("▶ " + s)
			default:
				item = infoStyle.Render("○ " + s)
			}
			parts = append(parts, item)
		}
		return strings.Join(parts, infoStyle.Render("  ·  "))
	}

	// stepWelcome is not part of the wizard steps — offset by 1
	stepNames := []string{"Path", "Type", "Provider", "Model", "Components", "Review"}
	stepEnums := []interactiveStep{stepPath, stepProjectType, stepProvider, stepModel, stepComponents, stepConfirm}

	var parts []string
	for i, s := range stepNames {
		idx := stepEnums[i]
		var item string
		switch {
		case idx < m.step:
			item = successStyle.Render("● " + s)
		case idx == m.step:
			item = titleStyle.Render("▶ " + s)
		default:
			item = infoStyle.Render("○ " + s)
		}
		parts = append(parts, item)
	}

	return strings.Join(parts, infoStyle.Render("  ·  "))
}

func (m setupModel) renderList(items []string, selected int) string {
	var rendered []string
	for idx, item := range items {
		prefix := "  "
		s := itemStyle

		if idx == selected {
			prefix = "▸ "
			s = selectedItemStyle
		}

		rendered = append(rendered, s.Render(prefix+item))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

func (m setupModel) renderFooter() string {
	var keys []string

	switch m.step {
	case stepPath:
		keys = []string{"Enter", "next", "ctrl+f", "browse", "ctrl+b", "back", "ctrl+q", "quit"}
	case stepProjectType, stepProvider:
		keys = []string{"↑↓", "move", "Enter", "next", "b", "back", "q", "quit"}
	case stepComponents:
		keys = []string{"↑↓", "move", "Space", "toggle", "Enter", "next", "b", "back", "q", "quit"}
	case stepConfirm:
		keys = []string{"Enter", "confirm", "n/b", "back", "q", "quit"}
	case stepSkillSelect:
		keys = []string{"↑↓", "move", "Space", "toggle", "a", "all", "n", "none", "Enter", "next", "b", "back"}
	case stepSkillConfirm:
		keys = []string{"Enter", "confirm", "n/b", "back", "q", "quit"}
	case stepAgentType:
		keys = []string{"↑↓", "move", "Enter", "select", "q/esc", "cancel"}
	case stepAgentName, stepAgentDescription, stepAgentPrompt:
		keys = []string{"Enter", "next", "b", "back", "q/esc", "cancel"}
	case stepAgentTools:
		keys = []string{"↑↓", "move", "Space", "toggle", "Enter", "next", "b", "back", "q/esc", "cancel"}
	case stepAgentConfirm:
		keys = []string{"Enter/y", "create", "n/b", "back", "q/esc", "cancel"}
	case stepAgentList:
		keys = []string{"↑↓", "move", "Enter", "menu", "q", "back"}
	case stepAgentMenu:
		keys = []string{"↑↓", "move", "Enter", "select", "q", "back"}
	case stepAgentView:
		keys = []string{"Enter/q", "back"}
	case stepAgentEdit:
		keys = []string{"↑↓", "field", "Enter", "save", "q", "cancel"}
	case stepAgentDeleteConfirm:
		keys = []string{"y", "confirm", "n", "cancel"}
	case stepFileBrowser:
		keys = []string{"↑↓", "move", "Enter", "select", "ctrl+l", "open", "ctrl+b", "up", "ctrl+q", "back"}
	}

	var helpParts []string
	for i := 0; i < len(keys); i += 2 {
		keyStr := keys[i]
		action := keys[i+1]
		helpParts = append(helpParts, helpStyle.Render(keyStr)+subtitleStyle.Render(" "+action))
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("  " + strings.Join(helpParts, " • "))
}

func (m setupModel) renderInstalling() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render(fmt.Sprintf("%s YWAI...", m.currentProgressVerb()))

	parts := []string{
		header,
		"",
		m.spinner.View() + " " + m.pulseLabel("Working..."),
	}

	if m.installTotal > 0 {
		bar := m.installBar
		if m.width > 0 {
			w := m.width / 2
			if w < 24 {
				w = 24
			}
			if w > 60 {
				w = 60
			}
			bar.Width = w
		}
		parts = append(parts,
			"",
			bar.View(),
			"",
			infoStyle.Render(fmt.Sprintf("%d/%d complete", m.installProgress, m.installTotal)),
		)
	}

	if m.installCurrent != "" {
		parts = append(parts,
			"",
			titleStyle.Render(m.pulseGlyph()+" Current: "+m.installCurrent),
		)
	}

	if len(m.installLogs) > 0 {
		maxW := 72
		if m.width > 0 {
			maxW = m.width / 2
			if maxW < 40 {
				maxW = 40
			}
		}
		parts = append(parts,
			"",
			boxStyle.Width(maxW).Render(strings.Join(m.installLogs, "\n")),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		lipgloss.JoinVertical(lipgloss.Left, parts...),
		"",
		infoStyle.Render(fmt.Sprintf("Please wait while YWAI %s your environment...", strings.ToLower(m.currentActionVerb())+"s")),
		"",
		helpStyle.Render("This can take a moment depending on downloads and local tools."),
	)
}

func (m setupModel) renderDone() string {
	if m.installErr != nil {
		icon := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("167")).
			Render("✗")

		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("167")).
			Render(func() string {
				if m.skillInstallMode {
					return "Skill Installation Failed"
				}
				if m.updateMode {
					return "Update Failed"
				}
				return "Setup Failed"
			}())

		message := errorStyle.Render(m.installErr.Error())

		return lipgloss.JoinVertical(
			lipgloss.Center,
			"",
			icon,
			"",
			title,
			"",
			message,
			"",
			helpStyle.Render("Press Enter or q to close"),
		)
	}

	icon := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("84")).
		Render("✓")

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render(func() string {
			if m.updateMode {
				return "Update Complete!"
			}
			return "Setup Complete!"
		}())

	message := infoStyle.Render(func() string {
		if m.skillInstallMode {
			return "Selected skills have been installed successfully."
		}
		if m.updateMode {
			return "YWAI has been updated successfully."
		}
		return "YWAI has been installed successfully."
	}())

	return lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		icon,
		"",
		title,
		"",
		message,
		"",
		helpStyle.Render("Press Enter or q to close"),
	)
}

func (m setupModel) renderQuitScreen() string {
	return lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		errorStyle.Render("✧ Setup cancelled"),
		"",
		infoStyle.Render("No changes were made to your system."),
	)
}
