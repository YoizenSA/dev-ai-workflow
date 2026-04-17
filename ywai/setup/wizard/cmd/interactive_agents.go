package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m setupModel) updateAgentType(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.agentTypeIdx > 0 {
				m.agentTypeIdx--
			}
		case "down", "j":
			if m.agentTypeIdx < len(m.agentTypeOptions)-1 {
				m.agentTypeIdx++
			}
		case "enter":
			m.step = stepAgentName
			m.agentNameInput.Focus()
		case "q", "esc":
			m.step = stepWelcome
		}
	}
	return m, nil
}

func (m setupModel) updateAgentName(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			value := strings.TrimSpace(m.agentNameInput.Value())
			if value == "" {
				return m, nil
			}
			m.step = stepAgentDescription
			m.agentDescInput.Focus()
		case "b":
			m.step = stepAgentType
		}
	}
	var cmd tea.Cmd
	m.agentNameInput, cmd = m.agentNameInput.Update(msg)
	return m, cmd
}

func (m setupModel) updateAgentDescription(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.step = stepAgentPrompt
			m.agentPromptInput.Focus()
		case "b":
			m.step = stepAgentName
			m.agentNameInput.Focus()
		}
	}
	var cmd tea.Cmd
	m.agentDescInput, cmd = m.agentDescInput.Update(msg)
	return m, cmd
}

func (m setupModel) updateAgentPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.step = stepAgentTools
		case "b":
			m.step = stepAgentDescription
			m.agentDescInput.Focus()
		}
	}
	var cmd tea.Cmd
	m.agentPromptInput, cmd = m.agentPromptInput.Update(msg)
	return m, cmd
}

func (m setupModel) updateAgentTools(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.agentToolCursor > 0 {
				m.agentToolCursor--
			}
		case "down", "j":
			if m.agentToolCursor < len(m.agentToolNames)-1 {
				m.agentToolCursor++
			}
		case " ":
			m.agentToolValues[m.agentToolCursor] = !m.agentToolValues[m.agentToolCursor]
		case "enter":
			m.step = stepAgentConfirm
		case "b":
			m.step = stepAgentPrompt
			m.agentPromptInput.Focus()
		}
	}
	return m, nil
}

func (m setupModel) updateAgentConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y":
			err := m.createAgentFile()
			if err != nil {
				m.agentError = err
			} else {
				m.agentCreated = true
			}
			m.step = stepAgentDone
		case "n", "b":
			m.step = stepAgentTools
		case "q", "esc":
			m.cancel = true
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m setupModel) createAgentFile() error {
	name := strings.TrimSpace(m.agentNameInput.Value())
	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}

	// Normalize name for filename
	filename := strings.ToLower(name)
	filename = strings.ReplaceAll(filename, " ", "-")
	filename = strings.ReplaceAll(filename, "_", "-")

	description := strings.TrimSpace(m.agentDescInput.Value())
	if description == "" {
		description = fmt.Sprintf("Agent %s", name)
	}

	prompt := strings.TrimSpace(m.agentPromptInput.Value())
	if prompt == "" {
		prompt = fmt.Sprintf("You are %s, a helpful assistant.", name)
	}

	agentType := m.agentTypeOptions[m.agentTypeIdx]

	// Build tools config
	toolsConfig := ""
	toolsEnabled := []string{}
	for i, enabled := range m.agentToolValues {
		if enabled {
			toolsEnabled = append(toolsEnabled, m.agentToolNames[i])
		}
	}

	if len(toolsEnabled) > 0 {
		toolsConfig = "\ntools:\n"
		for _, tool := range m.agentToolNames {
			enabled := false
			for _, e := range toolsEnabled {
				if e == tool {
					enabled = true
					break
				}
			}
			toolsConfig += fmt.Sprintf("  %s: %t\n", tool, enabled)
		}
	}

	content := fmt.Sprintf(`---
description: %s
mode: %s%s---
%s
`, description, agentType, toolsConfig, prompt)

	// Write the new agent to every managed destination at once so it's
	// immediately visible to OpenCode, Claude and Copilot.
	destDirs := []string{
		agentOpenCodeDir(),
		agentClaudeDir(),
		agentCopilotDir(),
	}
	for _, dir := range destDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create agents directory %s: %w", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, filename+".md"), []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write agent file under %s: %w", dir, err)
		}
	}

	return nil
}

// loadAgentList loads all agents from the agents directory.
func (m setupModel) loadAgentList() []AgentInfo {
	var agents []AgentInfo

	// Single source of truth: OpenCode singular "agent" directory. The
	// create/save/edit/delete pipeline keeps every managed destination in
	// sync so listing from OpenCode is sufficient.
	agentsDir := agentOpenCodeDir()

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return agents
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		info := AgentInfo{
			Name:        name,
			Description: "",
			Mode:        "",
		}

		// Try to read the file to get description and mode
		content, err := os.ReadFile(filepath.Join(agentsDir, entry.Name()))
		if err == nil {
			contentStr := string(content)
			// Parse frontmatter
			if strings.HasPrefix(contentStr, "---") {
				endIdx := strings.Index(contentStr[3:], "---")
				if endIdx > 0 {
					frontmatter := contentStr[3 : endIdx+3]
					// Extract description
					if descIdx := strings.Index(frontmatter, "description:"); descIdx >= 0 {
						lines := strings.Split(frontmatter[descIdx:], "\n")
						if len(lines) > 0 {
							desc := strings.TrimPrefix(lines[0], "description:")
							info.Description = strings.TrimSpace(desc)
						}
					}
					// Extract mode
					if modeIdx := strings.Index(frontmatter, "mode:"); modeIdx >= 0 {
						lines := strings.Split(frontmatter[modeIdx:], "\n")
						if len(lines) > 0 {
							mode := strings.TrimPrefix(lines[0], "mode:")
							info.Mode = strings.TrimSpace(mode)
						}
					}
				}
			}
		}

		agents = append(agents, info)
	}

	return agents
}

func (m setupModel) updateAgentList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.agentListCursor > 0 {
				m.agentListCursor--
			}
		case "down", "j":
			if m.agentListCursor < len(m.agentList)-1 {
				m.agentListCursor++
			}
		case "enter":
			if len(m.agentList) > 0 && m.agentListCursor < len(m.agentList) {
				m.agentSelected = m.agentList[m.agentListCursor].Name
				m.agentMenuCursor = 0
				m.step = stepAgentMenu
			}
		case "q", "esc":
			m.step = stepWelcome
		}
	}
	return m, nil
}

func (m setupModel) updateAgentMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.agentMenuCursor > 0 {
				m.agentMenuCursor--
			}
		case "down", "j":
			if m.agentMenuCursor < len(m.agentMenuOptions)-1 {
				m.agentMenuCursor++
			}
		case "enter":
			switch m.agentMenuOptions[m.agentMenuCursor] {
			case "View":
				content, _ := m.loadAgentContent(m.agentSelected)
				m.agentViewContent = content
				m.step = stepAgentView
			case "Edit":
				// Load agent data into edit fields
				m.loadAgentForEdit(m.agentSelected)
				m.step = stepAgentEdit
			case "Delete":
				m.agentToDelete = m.agentSelected
				m.step = stepAgentDeleteConfirm
			case "Back":
				m.step = stepAgentList
			}
		case "q", "esc":
			m.step = stepAgentList
		}
	}
	return m, nil
}

func (m setupModel) updateAgentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "enter":
			m.step = stepAgentMenu
		}
	}
	return m, nil
}

func (m setupModel) updateAgentEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if isKey {
		// Global keys (active on any field).
		switch keyMsg.String() {
		case "ctrl+s":
			if err := m.saveAgentEdit(m.agentSelected); err == nil {
				m.agentList = m.loadAgentList()
				m.step = stepAgentMenu
			}
			return m, tea.ClearScreen
		case "esc":
			m.step = stepAgentMenu
			return m, tea.ClearScreen
		case "tab":
			m.agentEditField = (m.agentEditField + 1) % 3
			m.refocusAgentEditField()
			return m, nil
		case "shift+tab":
			m.agentEditField = (m.agentEditField + 2) % 3
			m.refocusAgentEditField()
			return m, nil
		}

		// Tools field has its own keyboard (left/right to pick, space to toggle).
		// While on Description or Prompt we still let ↑↓ switch fields if the
		// user prefers them over Tab.
		if m.agentEditField == 2 {
			switch keyMsg.String() {
			case "left", "h":
				if m.agentToolCursor > 0 {
					m.agentToolCursor--
				}
				return m, nil
			case "right", "l":
				if m.agentToolCursor < len(m.agentToolNames)-1 {
					m.agentToolCursor++
				}
				return m, nil
			case "up", "k":
				m.agentEditField = 1
				m.refocusAgentEditField()
				return m, nil
			case "down", "j":
				// Already last field; wrap or ignore.
				return m, nil
			case " ":
				if m.agentToolCursor >= 0 && m.agentToolCursor < len(m.agentToolValues) {
					m.agentToolValues[m.agentToolCursor] = !m.agentToolValues[m.agentToolCursor]
				}
				return m, nil
			}
			// Any other key on tools field does nothing.
			return m, nil
		}
	}

	// Description or Prompt: forward keystrokes to the focused text input so
	// typing/editing actually works. (Previously this path was not wired, so
	// keystrokes were swallowed and the user could not change anything.)
	var cmd tea.Cmd
	if m.agentEditField == 0 {
		m.agentDescInput, cmd = m.agentDescInput.Update(msg)
	} else if m.agentEditField == 1 {
		m.agentPromptInput, cmd = m.agentPromptInput.Update(msg)
	}
	return m, cmd
}

// refocusAgentEditField blurs every editable input except the one matching
// the current field. Keeps the Charm cursor on the right text box.
func (m *setupModel) refocusAgentEditField() {
	m.agentDescInput.Blur()
	m.agentPromptInput.Blur()
	switch m.agentEditField {
	case 0:
		m.agentDescInput.Focus()
	case 1:
		m.agentPromptInput.Focus()
	}
}

// agentOpenCodeDir returns the directory where OpenCode stores global agents.
// Path note: OpenCode uses the singular "agent" directory. Earlier versions
// of this wizard wrote to "agents" (plural) which made Load/Save silently
// no-op because the file didn't exist. This helper centralizes the path so
// every call site stays aligned with the rest of the codebase
// (pkg/installer/globalagents/generator.go).
func agentOpenCodeDir() string {
	home, _ := os.UserHomeDir()
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfig, "opencode", "agent")
}

// agentClaudeDir and agentCopilotDir are the other two managed destinations.
func agentClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "agents")
}

func agentCopilotDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilot", "agents")
}

func (m *setupModel) loadAgentForEdit(name string) {
	filePath := filepath.Join(agentOpenCodeDir(), name+".md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	contentStr := string(content)

	description, tools, prompt := parseAgentMarkdown(contentStr)
	m.agentDescInput.SetValue(description)
	m.agentPromptInput.SetValue(prompt)
	if len(tools) > 0 {
		for idx, toolName := range m.agentToolNames {
			m.agentToolValues[idx] = tools[toolName]
		}
	}

	m.agentEditField = 0
	m.refocusAgentEditField()
}

// parseAgentMarkdown extracts (description, tools-map, prompt) from an agent
// markdown file with YAML frontmatter.
func parseAgentMarkdown(contentStr string) (string, map[string]bool, string) {
	tools := map[string]bool{}

	if !strings.HasPrefix(contentStr, "---") {
		return "", tools, strings.TrimSpace(contentStr)
	}

	endIdx := strings.Index(contentStr[3:], "---")
	if endIdx <= 0 {
		return "", tools, strings.TrimSpace(contentStr)
	}

	frontmatter := contentStr[3 : endIdx+3]
	promptStart := endIdx + 6 // past the closing "---\n"
	prompt := ""
	if promptStart < len(contentStr) {
		prompt = strings.TrimSpace(contentStr[promptStart:])
	}

	description := ""
	if descIdx := strings.Index(frontmatter, "description:"); descIdx >= 0 {
		rest := frontmatter[descIdx+len("description:"):]
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			description = strings.TrimSpace(rest[:nl])
		} else {
			description = strings.TrimSpace(rest)
		}
	}

	// Very small YAML parser just for "tools:" section. Accepts both inline
	// list ("tools: [read, write]") and block list ("tools:\n  read: true\n
	// write: true"). Anything we don't recognize is ignored.
	if toolsIdx := strings.Index(frontmatter, "tools:"); toolsIdx >= 0 {
		rest := frontmatter[toolsIdx+len("tools:"):]
		if inlineEnd := strings.Index(rest, "\n"); inlineEnd >= 0 {
			inline := strings.TrimSpace(rest[:inlineEnd])
			if strings.HasPrefix(inline, "[") && strings.HasSuffix(inline, "]") {
				body := strings.TrimSpace(inline[1 : len(inline)-1])
				for _, t := range strings.Split(body, ",") {
					name := strings.TrimSpace(strings.Trim(t, `"'`))
					if name != "" {
						tools[name] = true
					}
				}
			} else if inline == "" {
				// Block form — walk subsequent indented lines.
				for _, line := range strings.Split(rest[inlineEnd+1:], "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed == "" || !strings.HasPrefix(line, "  ") {
						break
					}
					colon := strings.Index(trimmed, ":")
					if colon <= 0 {
						continue
					}
					name := strings.TrimSpace(trimmed[:colon])
					val := strings.TrimSpace(trimmed[colon+1:])
					tools[name] = strings.EqualFold(val, "true")
				}
			}
		}
	}

	return description, tools, prompt
}

func (m *setupModel) saveAgentEdit(name string) error {
	openCodePath := filepath.Join(agentOpenCodeDir(), name+".md")

	// Keep the existing "mode:" value (subagent / primary) across edits.
	mode := "subagent"
	if existingContent, err := os.ReadFile(openCodePath); err == nil {
		contentStr := string(existingContent)
		if strings.HasPrefix(contentStr, "---") {
			if endIdx := strings.Index(contentStr[3:], "---"); endIdx > 0 {
				frontmatter := contentStr[3 : endIdx+3]
				if modeIdx := strings.Index(frontmatter, "mode:"); modeIdx >= 0 {
					rest := frontmatter[modeIdx+len("mode:"):]
					if nl := strings.Index(rest, "\n"); nl >= 0 {
						mode = strings.TrimSpace(rest[:nl])
					}
				}
			}
		}
	}

	description := strings.TrimSpace(m.agentDescInput.Value())
	if description == "" {
		description = fmt.Sprintf("Agent %s", name)
	}

	prompt := strings.TrimSpace(m.agentPromptInput.Value())
	if prompt == "" {
		prompt = fmt.Sprintf("You are %s, a helpful assistant.", name)
	}

	toolsBlock := renderAgentToolsBlock(m.agentToolNames, m.agentToolValues)

	content := fmt.Sprintf(`---
description: %s
mode: %s
%s---
%s
`, description, mode, toolsBlock, prompt)

	// Write to every managed destination so the edit stays consistent across
	// the three supported AI assistants (OpenCode, Claude, Copilot). Cursor
	// and Gemini are intentionally excluded per the supported-providers
	// scoping in v6.0.3-beta.5+.
	destDirs := []string{
		agentOpenCodeDir(),
		agentClaudeDir(),
		agentCopilotDir(),
	}

	var firstErr error
	for _, dir := range destDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		destPath := filepath.Join(dir, name+".md")
		if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// renderAgentToolsBlock renders the tool selection as YAML block form. Returns
// "" when no tool is selected so the frontmatter stays minimal.
func renderAgentToolsBlock(names []string, values []bool) string {
	if len(names) == 0 {
		return ""
	}
	var anyTrue bool
	var b strings.Builder
	b.WriteString("tools:\n")
	for idx, n := range names {
		v := false
		if idx < len(values) {
			v = values[idx]
		}
		if v {
			anyTrue = true
		}
		b.WriteString(fmt.Sprintf("  %s: %t\n", n, v))
	}
	if !anyTrue {
		return ""
	}
	return b.String()
}

func (m setupModel) loadAgentContent(name string) (string, error) {
	filePath := filepath.Join(agentOpenCodeDir(), name+".md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (m setupModel) updateAgentDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			if m.agentToDelete != "" {
				m.deleteAgentFile(m.agentToDelete)
				m.agentList = m.loadAgentList()
				if m.agentListCursor >= len(m.agentList) {
					m.agentListCursor = len(m.agentList) - 1
					if m.agentListCursor < 0 {
						m.agentListCursor = 0
					}
				}
			}
			m.agentToDelete = ""
			m.step = stepAgentList
		case "n", "esc":
			m.agentToDelete = ""
			m.step = stepAgentList
		}
	}
	return m, nil
}

func (m setupModel) deleteAgentFile(name string) error {
	// Remove the agent from every managed destination so it disappears from
	// OpenCode, Claude and Copilot atomically. os.Remove failures are
	// silently ignored (agent may not have existed in every target).
	for _, dir := range []string{agentOpenCodeDir(), agentClaudeDir(), agentCopilotDir()} {
		_ = os.Remove(filepath.Join(dir, name+".md"))
	}
	return nil
}

func (m setupModel) renderAgentTypeStep() string {
	box := activeBoxStyle.Render("Agent Type")

	options := []string{
		"primary  - Main agent (switch with Tab)",
		"subagent - Specialized agent (invoke with @)",
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render("Select agent type:"),
		"",
		m.renderList(options, m.agentTypeIdx),
	)
}

func (m setupModel) renderAgentNameStep() string {
	box := activeBoxStyle.Render("Agent Name")

	inputView := m.agentNameInput.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")).PaddingLeft(2).Render("Enter agent name:"),
		"",
		itemStyle.Render(inputView),
	)
}

func (m setupModel) renderAgentDescriptionStep() string {
	box := activeBoxStyle.Render("Agent Description")

	inputView := m.agentDescInput.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")).PaddingLeft(2).Render("Brief description of what this agent does:"),
		"",
		itemStyle.Render(inputView),
	)
}

func (m setupModel) renderAgentPromptStep() string {
	box := activeBoxStyle.Render("Agent Prompt")

	inputView := m.agentPromptInput.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")).PaddingLeft(2).Render("System prompt (role and behavior):"),
		"",
		itemStyle.Render(inputView),
	)
}

func (m setupModel) renderAgentToolsStep() string {
	box := activeBoxStyle.Render("Agent Tools")
	var items []string

	for idx, name := range m.agentToolNames {
		prefix := "[ ]"
		s := itemStyle

		if idx == m.agentToolCursor {
			s = selectedItemStyle
		}

		if m.agentToolValues[idx] {
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
		infoStyle.Render("Select tools (space to toggle):"),
		"",
		content,
	)
}

func (m setupModel) renderAgentConfirmStep() string {
	box := activeBoxStyle.Render("Confirm Agent Creation")

	name := strings.TrimSpace(m.agentNameInput.Value())
	if name == "" {
		name = m.agentNameInput.Placeholder
	}

	description := strings.TrimSpace(m.agentDescInput.Value())
	if description == "" {
		description = m.agentDescInput.Placeholder
	}

	agentType := m.agentTypeOptions[m.agentTypeIdx]

	lines := []string{
		infoStyle.Render("Ready to create agent:"),
		"",
		"  " + successStyle.Render("▶") + " Name: " + subtitleStyle.Render(name),
		"  " + successStyle.Render("▶") + " Type: " + subtitleStyle.Render(agentType),
		"  " + successStyle.Render("▶") + " Description: " + subtitleStyle.Render(description),
		"",
		infoStyle.Render("Tools enabled:"),
	}

	for idx, tool := range m.agentToolNames {
		if m.agentToolValues[idx] {
			lines = append(lines, "    "+successStyle.Render("✓")+" "+tool)
		}
	}

	lines = append(lines, "")
	lines = append(lines, infoStyle.Render("Press ")+titleStyle.Render("Enter")+" to create, "+titleStyle.Render("b/n")+" to go back")

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		content,
	)
}

func (m setupModel) renderAgentDone() string {
	if m.agentError != nil {
		icon := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("167")).
			Render("✗")

		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("167")).
			Render("Creation Failed")

		message := errorStyle.Render(m.agentError.Error())

		return lipgloss.JoinVertical(
			lipgloss.Center,
			"",
			icon,
			"",
			title,
			"",
			message,
		)
	}

	icon := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("84")).
		Render("✓")

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render("Agent Created!")

	name := strings.TrimSpace(m.agentNameInput.Value())
	message := infoStyle.Render(fmt.Sprintf("Agent '%s' has been created for:", name))

	locations := []string{
		successStyle.Render("✓") + " OpenCode",
		successStyle.Render("✓") + " Copilot",
	}

	return lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		icon,
		"",
		title,
		"",
		message,
		"",
		lipgloss.JoinVertical(lipgloss.Left, locations...),
	)
}

func (m setupModel) renderAgentListStep() string {
	box := activeBoxStyle.Render("Global Agents")

	if len(m.agentList) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			box,
			"",
			infoStyle.Render("No agents found."),
			"",
			helpStyle.Render("Press q to go back"),
		)
	}

	var items []string
	for idx, agent := range m.agentList {
		prefix := "  "
		s := itemStyle

		if idx == m.agentListCursor {
			prefix = "▸ "
			s = selectedItemStyle
		}

		name := agent.Name
		if agent.Mode != "" {
			name = name + " [" + agent.Mode + "]"
		}

		desc := agent.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		line := name
		if desc != "" {
			line = line + " - " + infoStyle.Render(desc)
		}

		items = append(items, s.Render(prefix+line))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render("Your agents (d/x to delete, q to go back):"),
		"",
		content,
	)
}

func (m setupModel) renderAgentDeleteConfirmStep() string {
	box := activeBoxStyle.Render("Delete Agent")

	lines := []string{
		infoStyle.Render("Are you sure you want to delete this agent?"),
		"",
		"  " + errorStyle.Render("⚠") + " " + titleStyle.Render(m.agentToDelete),
		"",
		infoStyle.Render("This will remove the agent from both OpenCode and Copilot."),
		"",
		helpStyle.Render("Press y to confirm, n to cancel"),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		content,
	)
}

func (m setupModel) renderAgentMenuStep() string {
	box := activeBoxStyle.Render("Agent Menu")

	var items []string
	for idx, option := range m.agentMenuOptions {
		prefix := "  "
		s := itemStyle

		if idx == m.agentMenuCursor {
			prefix = "▸ "
			s = selectedItemStyle
		}

		items = append(items, s.Render(prefix+option))
	}

	menu := lipgloss.JoinVertical(lipgloss.Left, items...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render(fmt.Sprintf("Agent: %s", m.agentSelected)),
		"",
		menu,
	)
}

func (m setupModel) renderAgentViewStep() string {
	box := activeBoxStyle.Render("View Agent")

	// Split content into lines and limit to screen height
	lines := strings.Split(m.agentViewContent, "\n")
	var contentLines []string
	for i, line := range lines {
		if i >= 20 { // Limit to 20 lines
			contentLines = append(contentLines, "...")
			break
		}
		contentLines = append(contentLines, line)
	}

	content := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Render(strings.Join(contentLines, "\n"))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render(fmt.Sprintf("Agent: %s", m.agentSelected)),
		"",
		content,
		"",
		helpStyle.Render("Press Enter or q to go back"),
	)
}

func (m setupModel) renderAgentEditStep() string {
	box := activeBoxStyle.Render("Edit Agent")

	// Build the Tools line as interactive checkboxes so the user can toggle
	// them. The cursor sits on the currently highlighted tool.
	var toolParts []string
	for idx, name := range m.agentToolNames {
		marker := "[ ]"
		if idx < len(m.agentToolValues) && m.agentToolValues[idx] {
			marker = "[✓]"
		}
		text := fmt.Sprintf("%s %s", marker, name)
		if m.agentEditField == 2 && idx == m.agentToolCursor {
			toolParts = append(toolParts, selectedItemStyle.Render(text))
		} else {
			toolParts = append(toolParts, itemStyle.Render(text))
		}
	}
	toolsLine := strings.Join(toolParts, "  ")

	fields := []struct {
		label string
		input string
	}{
		{"Description:", m.agentDescInput.View()},
		{"Prompt:", m.agentPromptInput.View()},
		{"Tools:", toolsLine},
	}

	var items []string
	for idx, field := range fields {
		prefix := "  "
		s := itemStyle

		if idx == m.agentEditField {
			prefix = "▸ "
			s = selectedItemStyle
		}

		line := fmt.Sprintf("%s%s\n    %s", prefix, field.label, field.input)
		items = append(items, s.Render(line))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	hint := helpStyle.Render(
		"Tab next field  •  Type to edit description/prompt  •  ←→ pick tool  •  Space toggle tool\n" +
			"Ctrl+S save  •  Esc cancel",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		"",
		infoStyle.Render(fmt.Sprintf("Editing: %s", m.agentSelected)),
		"",
		content,
		"",
		hint,
	)
}
