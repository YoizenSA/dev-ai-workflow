package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m setupModel) updateWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.welcomeIdx > 0 {
				m.welcomeIdx--
			}
		case "down", "j":
			if m.welcomeIdx < len(m.welcomeOptions)-1 {
				m.welcomeIdx++
			}
		case "enter":
			switch m.welcomeIdx {
			case 0: // Install
				m.updateMode = false
				m.skillInstallMode = false
				m.step = stepPath
				m.pathInput.Focus()
			case 1: // Update — same flow for now
				m.updateMode = true
				m.skillInstallMode = false
				m.step = stepPath
				m.pathInput.Focus()
			case 2: // Install repo skills
				m.updateMode = false
				m.skillInstallMode = true
				m.step = stepPath
				m.pathInput.Focus()
			case 3: // Update global tools
				m.globalToolNames = []string{
					"GA (Guardian Agent)",
					"SDD Orchestrator skills",
					"Global agents & skills",
					"Engram CLI",
					"Context7 MCP",
				}
				m.globalToolValues = []bool{true, true, true, true, true}
				m.globalToolCursor = 0
				m.globalToolDone = false
				m.step = stepGlobalTools
			case 4: // Create global agent
				m.step = stepAgentType
			case 5: // List global agents
				m.agentList = m.loadAgentList()
				m.agentListCursor = 0
				m.step = stepAgentList
			case 6: // Quit
				m.cancel = true
				m.quitting = true
				return m, tea.Quit
			}
		case "q", "esc":
			m.cancel = true
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m setupModel) renderWelcomeStep() string {
	subtitle := bodyStyle.Render("Set up AI workflows for a project in a guided way")
	options := []menuOption{
		{Title: m.welcomeOptions[0], Description: "Best for first-time setup in a repository"},
		{Title: m.welcomeOptions[1], Description: "Refresh an existing setup and re-apply managed files"},
		{Title: m.welcomeOptions[2], Description: "Install only the YWAI skills that are still missing in this repo"},
		{Title: m.welcomeOptions[3], Description: "Update GA, SDD, Engram, Context7, global agents — no repo needed"},
		{Title: m.welcomeOptions[4], Description: "Create a reusable agent for OpenCode / Copilot"},
		{Title: m.welcomeOptions[5], Description: "View, edit, or delete existing global agents"},
		{Title: m.welcomeOptions[6], Description: "Exit without making changes"},
	}

	var items []string
	for idx, opt := range options {
		line := opt.Title + "\n" + captionStyle.Render("   "+opt.Description)
		if idx == m.welcomeIdx {
			items = append(items, selectedItemStyle.Render("▸ "+line))
		} else {
			items = append(items, itemStyle.Render("  "+line))
		}
	}

	menu := lipgloss.JoinVertical(lipgloss.Left, items...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		subtitle,
		"",
		h2Style.Render("What do you want to do today?"),
		"",
		menu,
	)
}
