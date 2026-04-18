package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	syncpkg "github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/sync"
	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// isComponentLocked reports whether a component cannot be toggled off. With
// the granular 10-item list nothing is hard-locked anymore — every component
// is individually togglable. The function is kept as a single point of truth
// in case we need to reintroduce lock rules in the future (e.g. "require GA
// when provider is OpenCode").
func (m setupModel) isComponentLocked(idx int) bool {
	return false
}

func detectProjectTypeFromPath(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	packageJsonPath := filepath.Join(target, "package.json")
	if data, err := os.ReadFile(packageJsonPath); err == nil {
		content := string(data)
		if strings.Contains(content, "@nestjs/core") || strings.Contains(content, "nestjs") {
			if strings.Contains(content, "@angular") || strings.Contains(content, "angular") {
				return "nest-angular"
			}
			if strings.Contains(content, "react") || strings.Contains(content, "@react") {
				return "nest-react"
			}
			return "nest"
		}
		return "generic"
	}

	if _, err := os.Stat(filepath.Join(target, "pyproject.toml")); err == nil {
		return "python"
	}

	if matches, _ := filepath.Glob(filepath.Join(target, "*.csproj")); len(matches) > 0 {
		return "dotnet"
	}

	if data, err := os.ReadFile(filepath.Join(target, "Dockerfile")); err == nil {
		content := string(data)
		switch {
		case strings.Contains(content, "python"), strings.Contains(content, "pip"):
			return "python"
		case strings.Contains(content, "dotnet"), strings.Contains(content, "nuget"):
			return "dotnet"
		case strings.Contains(content, "node"), strings.Contains(content, "npm"):
			return "nest"
		}
	}

	return "generic"
}

func loadInstallableSkillsForPath(target string) ([]syncpkg.SkillInfo, error) {
	logger := ui.NewLogger(true)
	s := syncpkg.New(&syncpkg.SyncFlags{}, logger, target)
	missing, _, _, err := s.GetInstallableSkills()
	if err != nil {
		return nil, err
	}
	return missing, nil
}

func (m setupModel) updatePath(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			value := strings.TrimSpace(m.pathInput.Value())
			if value == "" {
				m.err = fmt.Errorf("project path cannot be empty")
				return m, nil
			}
			if m.skillInstallMode {
				skills, err := loadInstallableSkillsForPath(value)
				if err != nil {
					m.err = err
					return m, nil
				}
				m.skillOptions = skills
				m.skillValues = make([]bool, len(skills))
				for idx := range m.skillValues {
					m.skillValues[idx] = true
				}
				m.skillCursor = 0
				m.skillLoadError = nil
				m.step = stepSkillSelect
				m.pathInput.Blur()
				m.err = nil
				return m, nil
			}
			detected := detectProjectTypeFromPath(value)
			if detected != "" {
				for idx, pt := range m.projectTypeValues {
					if pt == detected {
						m.projectTypeIdx = idx
						break
					}
				}
			}
			m.step = stepProjectType
			m.pathInput.Blur()
			m.err = nil
			return m, nil
		case "ctrl+b":
			m.step = stepWelcome
			m.pathInput.Blur()
			return m, nil
		case "ctrl+c", "ctrl+q":
			m.cancel = true
			m.quitting = true
			m.pathInput.Blur()
			return m, tea.Quit
		case "ctrl+f":
			// Open file browser
			m.fileBrowserDir, _ = os.Getwd()
			if m.pathInput.Value() != "" {
				m.fileBrowserDir = m.pathInput.Value()
			}
			m.fileBrowserEntries = m.loadFileBrowser(m.fileBrowserDir)
			m.fileBrowserCursor = 0
			m.pathInput.Blur()
			m.step = stepFileBrowser
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m setupModel) updateProjectType(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.projectTypeIdx > 0 {
				m.projectTypeIdx--
			}
		case "down", "j":
			if m.projectTypeIdx < len(m.projectTypeLabels)-1 {
				m.projectTypeIdx++
			}
		case "enter":
			m.step = stepProvider
		case "b":
			m.step = stepPath
			m.pathInput.Focus()
		}
	}
	return m, nil
}

func (m setupModel) updateProvider(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.providerIdx > 0 {
				m.providerIdx--
			}
		case "down", "j":
			if m.providerIdx < len(m.providerLabels)-1 {
				m.providerIdx++
			}
		case "enter":
			m.step = stepModel
		case "b":
			m.step = stepProjectType
		}
	}
	return m, nil
}

func (m setupModel) updateModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modelCustom {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				m.step = stepInstallMode
				m.modelInput.Blur()
			case "esc":
				m.modelCustom = false
				m.modelInput.Blur()
			}
		}
		var cmd tea.Cmd
		m.modelInput, cmd = m.modelInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.modelPresetIdx > 0 {
				m.modelPresetIdx--
			}
		case "down", "j":
			if m.modelPresetIdx < len(m.modelPresets)-1 {
				m.modelPresetIdx++
			}
		case "enter":
			if m.modelPresetIdx == len(m.modelPresets)-1 {
				m.modelCustom = true
				m.modelInput.Focus()
			} else {
				m.step = stepInstallMode
			}
		case "b":
			m.step = stepProvider
		}
	}
	return m, nil
}

// updateInstallMode handles the "Install recommended? (Y/n)" screen. Enter on
// "All recommended" jumps straight to the Review step; Enter on "Custom"
// opens the 10-item component checklist.
func (m setupModel) updateInstallMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.installModeIdx > 0 {
				m.installModeIdx--
			}
		case "down", "j":
			if m.installModeIdx < len(m.installModeOptions)-1 {
				m.installModeIdx++
			}
		case "y", "Y":
			m.installModeIdx = 0
			m.step = stepConfirm
		case "n", "N":
			m.installModeIdx = 1
			m.step = stepComponents
		case "enter":
			if m.installModeIdx == 0 {
				m.step = stepConfirm
			} else {
				m.step = stepComponents
			}
		case "b":
			m.step = stepModel
		}
	}
	return m, nil
}

func (m setupModel) updateComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.componentCursor > 0 {
				m.componentCursor--
			}
		case "down", "j":
			if m.componentCursor < len(m.componentNames)-1 {
				m.componentCursor++
			}
		case " ":
			if !m.isComponentLocked(m.componentCursor) {
				m.componentValues[m.componentCursor] = !m.componentValues[m.componentCursor]
			}
		case "a":
			// 'a' = toggle "all on" / "all off" (keeps Biome and DryRun
			// opt-in even when selecting all).
			allSet := true
			for idx, v := range m.componentValues {
				if !v && !m.isComponentOptional(idx) {
					allSet = false
					break
				}
			}
			for idx := range m.componentValues {
				if m.isComponentOptional(idx) {
					continue // leave Biome / DryRun untouched
				}
				m.componentValues[idx] = !allSet
			}
		case "enter":
			m.step = stepConfirm
		case "b":
			m.step = stepInstallMode
		}
	}
	return m, nil
}

func (m setupModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y":
			return m.beginProjectInstallation()
		case "n", "b":
			// Go back to whichever screen the user came from.
			if m.installModeIdx == 0 {
				m.step = stepInstallMode
			} else {
				m.step = stepComponents
			}
		}
	}
	return m, nil
}

// isComponentOptional reports whether a component index represents an opt-in
// toggle that "select-all" should leave untouched (Biome, Dry run).
func (m setupModel) isComponentOptional(idx int) bool {
	// Indices map to componentNames; Biome=8, DryRun=9.
	return idx == 8 || idx == 9
}

func (m setupModel) renderPathStep() string {
	box := activeBoxStyle.Render(m.currentModeLabel() + " • Project Directory")

	inputView := m.pathInput.View()
	if m.err != nil {
		inputView = inputView + "\n" + errorStyle.Render("⚠ "+m.err.Error())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")).PaddingLeft(2).Render(
			func() string {
				if m.skillInstallMode {
					return "Choose the repository where you want to install missing skills:"
				}
				return fmt.Sprintf("Choose the project folder where YWAI should %s:", m.currentActionVerb())
			}(),
		),
		"",
		helpStyle.Render("Tip: press ctrl+f to browse folders"),
		"",
		itemStyle.Render(inputView),
	)
}

func (m setupModel) renderProjectTypeStep() string {
	box := activeBoxStyle.Render(m.currentModeLabel() + " • Project Type")
	path := strings.TrimSpace(m.pathInput.Value())
	detected := detectProjectTypeFromPath(path)
	hint := infoStyle.Render("Pick the closest match for this repository.")
	if detected != "" {
		hint = infoStyle.Render("Detected from files: ") + titleStyle.Render(detected)
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		hint,
		"",
		m.renderList(m.projectTypeLabels, m.projectTypeIdx),
		"",
		helpStyle.Render(m.projectTypeHints[m.projectTypeIdx]),
	)
}

func (m setupModel) renderProviderStep() string {
	box := activeBoxStyle.Render(m.currentModeLabel() + " • AI Provider")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render("Select the main AI client your team will use:"),
		"",
		m.renderList(m.providerLabels, m.providerIdx),
		"",
		helpStyle.Render("OpenCode is the default and enables the most integrated workflow."),
	)
}

func (m setupModel) renderModelStep() string {
	box := activeBoxStyle.Render(m.currentModeLabel() + " • Default Model")

	if m.modelCustom {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			box,
			"",
			infoStyle.Render("Enter the model identifier (provider/model):"),
			"",
			itemStyle.Render(m.modelInput.View()),
			"",
			helpStyle.Render("Press Enter to confirm, Esc to go back to presets"),
		)
	}

	var items []string
	for idx, preset := range m.modelPresets {
		line := preset
		if idx == m.modelPresetIdx {
			items = append(items, selectedItemStyle.Render("▸ "+line))
		} else {
			items = append(items, itemStyle.Render("  "+line))
		}
	}

	menu := lipgloss.JoinVertical(lipgloss.Left, items...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render("Select the default AI model for SDD phases:"),
		"",
		menu,
		"",
		helpStyle.Render("This model will be used for all SDD phases. You can customize per-phase in sdd/config.yaml later."),
	)
}

func (m setupModel) getSelectedModel() string {
	if m.modelCustom && m.modelInput.Value() != "" {
		return m.modelInput.Value()
	}
	if m.modelPresetIdx == 0 {
		return ""
	}
	if m.modelPresetIdx == len(m.modelPresets)-1 {
		return m.modelInput.Value()
	}
	return m.modelPresets[m.modelPresetIdx]
}

// renderInstallModeStep presents the "Install recommended? (Y/n)" radio.
func (m setupModel) renderInstallModeStep() string {
	box := activeBoxStyle.Render(m.currentModeLabel() + " • Install Mode")

	var items []string
	for idx, label := range m.installModeOptions {
		prefix := "(  )"
		s := itemStyle
		if idx == m.installModeIdx {
			prefix = "( ● )"
			s = selectedItemStyle
		}
		items = append(items, s.Render(fmt.Sprintf("%s %s", prefix, label)))
	}

	hint := infoStyle.Render(
		"Press Y (or Enter on 'All recommended') to install every component with sane defaults.\n" +
			"Press N to pick each component individually (AGENTS.md, Skills, Commands, MCPs, GA,\n" +
			"Engram, Global agents, Hooks and Biome).",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		lipgloss.JoinVertical(lipgloss.Left, items...),
		"",
		hint,
	)
}

func (m setupModel) renderComponentsStep() string {
	box := activeBoxStyle.Render(m.currentModeLabel() + " • Components (Custom)")
	var items []string

	for idx, name := range m.componentNames {
		prefix := "[ ]"
		s := itemStyle

		if idx == m.componentCursor {
			s = selectedItemStyle
		}

		if m.componentValues[idx] {
			prefix = "[✓]"
		}

		line := fmt.Sprintf("%s %s", prefix, name)
		items = append(items, s.Render(line))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render("Toggle every component you want installed:"),
		"",
		helpStyle.Render("Space to toggle  •  A to select/deselect all  •  Enter to continue  •  B to go back"),
		"",
		content,
	)
}

func (m setupModel) renderConfirmStep() string {
	box := activeBoxStyle.Render(m.currentModeLabel() + " • Review")

	path := strings.TrimSpace(m.pathInput.Value())
	if path == "" {
		path = m.pathInput.Placeholder
	}

	projectType := m.projectTypeValues[m.projectTypeIdx]
	provider := m.providerValues[m.providerIdx]
	model := m.getSelectedModel()
	if model == "" {
		model = "(agent default)"
	}

	modeLabel := "All recommended"
	if m.installModeIdx == 1 {
		modeLabel = "Custom selection"
	}

	lines := []string{
		infoStyle.Render(fmt.Sprintf("Ready to %s YWAI in this project:", strings.ToLower(m.currentModeLabel()))),
		"",
		"  " + successStyle.Render("▶") + " Path: " + subtitleStyle.Render(path),
		"  " + successStyle.Render("▶") + " Type: " + subtitleStyle.Render(projectType),
		"  " + successStyle.Render("▶") + " Provider: " + subtitleStyle.Render(provider),
		"  " + successStyle.Render("▶") + " Model: " + subtitleStyle.Render(model),
		"  " + successStyle.Render("▶") + " Mode: " + subtitleStyle.Render(modeLabel),
		"",
		infoStyle.Render("What will be applied:"),
	}

	// Show the list of components. In "All recommended" mode we enumerate
	// the defaults (everything except Biome and DryRun); in "Custom" we
	// reflect the user's checkbox selection.
	for idx, name := range m.componentNames {
		enabled := false
		if m.installModeIdx == 0 {
			enabled = !m.isComponentOptional(idx)
		} else {
			enabled = m.componentValues[idx]
		}
		if enabled {
			lines = append(lines, "    "+successStyle.Render("✓")+" "+name)
		} else {
			lines = append(lines, "    "+helpStyle.Render("○ "+name))
		}
	}

	lines = append(lines, "")
	if m.updateMode {
		lines = append(lines, helpStyle.Render("Update mode refreshes managed files, skills, extensions, and GA/runtime setup."))
		lines = append(lines, "")
	}
	lines = append(lines, infoStyle.Render("Press ")+titleStyle.Render("Enter")+" to continue, "+titleStyle.Render("b/n")+" to go back")

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		content,
	)
}
