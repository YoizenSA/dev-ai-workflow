package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m setupModel) updateSkillSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.skillCursor > 0 {
				m.skillCursor--
			}
		case "down", "j":
			if m.skillCursor < len(m.skillOptions)-1 {
				m.skillCursor++
			}
		case " ":
			if len(m.skillValues) > 0 && m.skillCursor < len(m.skillValues) {
				m.skillValues[m.skillCursor] = !m.skillValues[m.skillCursor]
			}
		case "a":
			for idx := range m.skillValues {
				m.skillValues[idx] = true
			}
		case "n":
			for idx := range m.skillValues {
				m.skillValues[idx] = false
			}
		case "enter":
			m.step = stepSkillConfirm
		case "b":
			m.step = stepPath
			m.pathInput.Focus()
		}
	}
	return m, nil
}

func (m setupModel) updateSkillConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y":
			m.done = true
			return m, tea.Quit
		case "n", "b":
			m.step = stepSkillSelect
		}
	}
	return m, nil
}

func (m setupModel) renderSkillSelectStep() string {
	box := activeBoxStyle.Render("Install Missing Skills")

	if len(m.skillOptions) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			box,
			"",
			successStyle.Render("This repository already has all installable YWAI skills."),
			"",
			captionStyle.Render("Press b to go back and choose another path."),
		)
	}

	var items []string
	for idx, skill := range m.skillOptions {
		prefix := "[ ]"
		style := itemStyle
		if idx == m.skillCursor {
			style = selectedItemStyle
		}
		if idx < len(m.skillValues) && m.skillValues[idx] {
			prefix = "[x]"
		}
		desc := strings.TrimSpace(skill.Description)
		if desc == "" {
			desc = "No description"
		}
		items = append(items, style.Render(fmt.Sprintf("%s %s", prefix, skill.Name)))
		items = append(items, captionStyle.Render("    "+desc))
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		bodyStyle.Render("Select the missing skills you want to install in this repository:"),
		"",
		lipgloss.JoinVertical(lipgloss.Left, items...),
		"",
		captionStyle.Render("Space toggle • a select all • n clear all"),
	)
}

func (m setupModel) renderSkillConfirmStep() string {
	box := activeBoxStyle.Render("Review Skill Installation")
	var selected []string
	for idx, skill := range m.skillOptions {
		if idx < len(m.skillValues) && m.skillValues[idx] {
			selected = append(selected, skill.Name)
		}
	}

	lines := []string{
		h3Style.Render("Repository:"),
		"  " + bodyStyle.Render(strings.TrimSpace(m.pathInput.Value())),
		"",
		h3Style.Render("Skills to install:"),
	}
	if len(selected) == 0 {
		lines = append(lines, "  "+errorStyle.Render("No skills selected"))
		lines = append(lines, "", captionStyle.Render("Press b to go back and choose at least one skill."))
	} else {
		for _, skill := range selected {
			lines = append(lines, "  "+successStyle.Render("[x]")+" "+bodyStyle.Render(skill))
		}
		lines = append(lines, "", captionStyle.Render("YWAI will copy the selected skills, run skills/setup.sh, and try to sync AGENTS.md metadata."))
	}

	lines = append(lines, "", captionStyle.Render("Press ")+titleStyle.Render("Enter")+" to continue, "+titleStyle.Render("b/n")+" to go back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	)
}
