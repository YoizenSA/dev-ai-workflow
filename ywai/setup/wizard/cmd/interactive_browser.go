package main

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// File browser methods

func (m setupModel) loadFileBrowser(dir string) []os.DirEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []os.DirEntry{}
	}

	// Filter to show only directories and .md files
	var filtered []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".md") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (m setupModel) updateFileBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.fileBrowserCursor > 0 {
				m.fileBrowserCursor--
			}
		case "down", "j":
			if m.fileBrowserCursor < len(m.fileBrowserEntries)-1 {
				m.fileBrowserCursor++
			}
		case "enter":
			if m.fileBrowserCursor < len(m.fileBrowserEntries) {
				entry := m.fileBrowserEntries[m.fileBrowserCursor]
				m.pathInput.SetValue(filepath.Join(m.fileBrowserDir, entry.Name()))
				m.step = stepPath
				m.pathInput.Focus()
			} else {
				m.pathInput.SetValue(m.fileBrowserDir)
				m.step = stepPath
				m.pathInput.Focus()
			}
		case "ctrl+l":
			if m.fileBrowserCursor < len(m.fileBrowserEntries) {
				entry := m.fileBrowserEntries[m.fileBrowserCursor]
				if entry.IsDir() {
					// Navigate into directory
					m.fileBrowserDir = filepath.Join(m.fileBrowserDir, entry.Name())
					m.fileBrowserEntries = m.loadFileBrowser(m.fileBrowserDir)
					m.fileBrowserCursor = 0
				}
			}
		case "ctrl+b":
			// Go up one directory
			parent := filepath.Dir(m.fileBrowserDir)
			if parent != m.fileBrowserDir {
				m.fileBrowserDir = parent
				m.fileBrowserEntries = m.loadFileBrowser(m.fileBrowserDir)
				m.fileBrowserCursor = 0
			}
		case "ctrl+q", "esc":
			m.step = stepPath
			m.pathInput.Focus()
		}
	}
	return m, nil
}

func (m setupModel) renderFileBrowserStep() string {
	box := activeBoxStyle.Render("File Browser")

	// Current path
	pathLine := bodyStyle.Render("Current: " + monoStyle.Render(m.fileBrowserDir))

	if len(m.fileBrowserEntries) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			box,
			"",
			pathLine,
			"",
			bodyStyle.Render("No directories or .md files found."),
			"",
			captionStyle.Render("Press q to go back"),
		)
	}

	var items []string
	for idx, entry := range m.fileBrowserEntries {
		prefix := "  "
		s := itemStyle

		if idx == m.fileBrowserCursor {
			prefix = "▸ "
			s = selectedItemStyle
		}

		name := entry.Name()
		if entry.IsDir() {
			name = name + "/"
		}

		items = append(items, s.Render(prefix+name))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		pathLine,
		"",
		bodyStyle.Render("Enter selects the highlighted item. Use Ctrl+L to open a folder."),
		"",
		content,
		"",
		captionStyle.Render("Tip: press esc to go back without changing the path."),
	)
}
