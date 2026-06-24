package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// DelegationsFile is the default name of the delegations source-of-truth file,
// expected next to the agent profiles (ywai/agents/delegations.json).
const DelegationsFile = "delegations.json"

// DelegationRule is one row of the "Delegation Rules" table.
type DelegationRule struct {
	Action   string `json:"action"`
	Inline   string `json:"inline"`
	Delegate string `json:"delegate"`
}

// DelegationTrigger is one item of the "Mandatory Delegation Triggers" list.
type DelegationTrigger struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AgentDelegation is the per-agent entry: the task map + optional rule/triggers
// overrides + a skip flag to opt out of prompt rules entirely.
type AgentDelegation struct {
	Task      map[string]string   `json:"task,omitempty"`
	Rules     []DelegationRule    `json:"rules,omitempty"`
	Triggers  []DelegationTrigger `json:"triggers,omitempty"`
	SkipRules bool                `json:"skip_rules,omitempty"`
}

// DelegationsDoc is the full on-disk shape of delegations.json.
type DelegationsDoc struct {
	Defaults struct {
		Rules    []DelegationRule    `json:"rules"`
		Triggers []DelegationTrigger `json:"triggers"`
	} `json:"defaults"`
	Agents map[string]AgentDelegation `json:"agents"`
}

// LoadDelegations reads delegations.json next to the agent profiles. Returns an
// empty doc (not an error) when the file is absent so installs that pre-date
// this feature keep working.
func LoadDelegations(sourceDir string) (*DelegationsDoc, error) {
	path := filepath.Join(sourceDir, DelegationsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &DelegationsDoc{Agents: map[string]AgentDelegation{}}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc DelegationsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if doc.Agents == nil {
		doc.Agents = map[string]AgentDelegation{}
	}
	return &doc, nil
}

// RulesFor returns the effective rules for an agent: its override if set,
// otherwise the defaults. Empty (and hasRules=false) when SkipRules is set.
func (d *DelegationsDoc) RulesFor(agent string) ([]DelegationRule, bool) {
	a, ok := d.Agents[agent]
	if !ok || a.SkipRules {
		return nil, false
	}
	if len(a.Rules) > 0 {
		return a.Rules, true
	}
	if len(d.Defaults.Rules) > 0 {
		return d.Defaults.Rules, true
	}
	return nil, false
}

// TriggersFor returns the effective triggers for an agent (override or default).
func (d *DelegationsDoc) TriggersFor(agent string) []DelegationTrigger {
	a, ok := d.Agents[agent]
	if !ok || a.SkipRules {
		return nil
	}
	if len(a.Triggers) > 0 {
		return a.Triggers
	}
	return d.Defaults.Triggers
}

// ApplyDelegations writes the per-agent permission.task map from delegations
// into opencode.json, renders the rules+triggers into each agent's markdown
// prompt body, AND persists the full doc as agentsDir/delegations.json so the
// Orchestrator UI can read/write rules as structured JSON (never parsing .md).
// It is idempotent: re-running with the same doc leaves files unchanged.
//
// configPath is the opencode.json/.jsonc path. agentsDir is where the .md files
// live (e.g. ~/.config/opencode/agents).
func ApplyDelegations(configPath, agentsDir string, doc *DelegationsDoc) error {
	if doc == nil || len(doc.Agents) == 0 {
		return nil
	}

	if err := applyTaskMaps(configPath, doc); err != nil {
		return err
	}
	if err := applyRulesToMarkdown(agentsDir, doc); err != nil {
		return err
	}
	return persistDelegationsSidecar(agentsDir, doc)
}

// persistDelegationsSidecar writes the full doc to agentsDir/delegations.json
// so the UI has a structured read/write target instead of parsing markdown.
func persistDelegationsSidecar(agentsDir string, doc *DelegationsDoc) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal delegations sidecar: %w", err)
	}
	path := filepath.Join(agentsDir, DelegationsFile)
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// applyTaskMaps writes agent.<name>.permission.task from the doc into opencode.json.
func applyTaskMaps(configPath string, doc *DelegationsDoc) error {
	hasTask := false
	for _, a := range doc.Agents {
		if len(a.Task) > 0 {
			hasTask = true
			break
		}
	}
	if !hasTask {
		return nil
	}

	root := map[string]any{}
	if _, err := os.Stat(configPath); err == nil {
		r, err := config.ReadJSONC(configPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", configPath, err)
		}
		root = r
	} else {
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}

	agentsRaw, ok := root["agent"]
	if !ok {
		agentsRaw = map[string]any{}
		root["agent"] = agentsRaw
	}
	agents, ok := agentsRaw.(map[string]any)
	if !ok {
		agents = map[string]any{}
		root["agent"] = agents
	}

	applied := 0
	for name, ad := range doc.Agents {
		if len(ad.Task) == 0 {
			continue
		}
		entry, ok := agents[name].(map[string]any)
		if !ok {
			entry = map[string]any{}
			agents[name] = entry
		}

		permRaw, _ := entry["permission"].(map[string]any)
		if permRaw == nil {
			permRaw = map[string]any{}
		}
		obj := make(map[string]any, len(ad.Task))
		for k, v := range ad.Task {
			obj[k] = v
		}
		permRaw["task"] = obj
		entry["permission"] = permRaw
		applied++
	}

	if applied == 0 {
		return nil
	}
	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	fmt.Printf("  Applied delegation graph to %d agents\n", applied)
	return nil
}

// applyRulesToMarkdown renders the rules table + triggers list into each agent's
// .md prompt body (under a "### Delegation Rules" heading). Agents that opt out
// via SkipRules are left untouched. The markdown is GENERATED from the JSON, so
// there is no parser to break: the JSON is always the source of truth.
func applyRulesToMarkdown(agentsDir string, doc *DelegationsDoc) error {
	rendered := 0
	for name := range doc.Agents {
		rules, hasRules := doc.RulesFor(name)
		triggers := doc.TriggersFor(name)
		if !hasRules && len(triggers) == 0 {
			continue
		}
		path := filepath.Join(agentsDir, name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue // agent markdown not installed yet; skip
		}

		body := renderRulesSection(rules, triggers)
		updated := replaceMarkdownSection(string(data), "Delegation Rules", "###", body, true)
		if updated == string(data) {
			continue // nothing changed
		}
		_ = os.WriteFile(path+".bak", data, 0o644)
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			fmt.Printf("  Warning: failed to write delegation rules to %s: %v\n", path, err)
			continue
		}
		rendered++
	}
	if rendered > 0 {
		fmt.Printf("  Rendered delegation rules into %d agent prompts\n", rendered)
	}
	return nil
}

// renderRulesSection builds the markdown body for the Delegation Rules section
// (table + nested triggers list) from structured data.
func renderRulesSection(rules []DelegationRule, triggers []DelegationTrigger) string {
	var b strings.Builder
	b.WriteString("Core principle: **does this inflate my context without need?** If yes -> delegate. If no -> do it inline.\n\n")

	if len(rules) > 0 {
		b.WriteString("| Action | Inline | Delegate |\n")
		b.WriteString("| ------ | ------ | -------- |\n")
		for _, r := range rules {
			action := strings.ReplaceAll(r.Action, "|", "\\|")
			delegate := strings.ReplaceAll(r.Delegate, "|", "\\|")
			inline := r.Inline
			if inline == "" {
				inline = "No"
			}
			if delegate == "" {
				delegate = "No"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", action, inline, delegate))
		}
		b.WriteString("\nUse OpenCode's native `task` tool for delegated work.\n")
	}

	if len(triggers) > 0 {
		b.WriteString("\n#### Mandatory Delegation Triggers\n\n")
		b.WriteString("These gates are **non-skippable hard gates**, not recommendations.\n\n")
		b.WriteString("Semantic guard: **delegate** means using OpenCode's native `task` tool to invoke a configured sub-agent. Running local scripts, Python, or Bash inline is execution, not delegation.\n\n")
		for i, t := range triggers {
			name := strings.TrimSpace(t.Name)
			if name == "" {
				name = "Trigger"
			}
			b.WriteString(fmt.Sprintf("%d. **%s**: %s\n", i+1, name, strings.TrimSpace(t.Description)))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// replaceMarkdownSection replaces the body content under the heading headerText
// with newContent. If the heading does not exist, the section (heading + "\n\n"
// + newContent) is appended at the end of the content. (Re-exported here to
// avoid an import cycle with the kanban package's copy.)
func replaceMarkdownSection(content, headerText, headingPrefix, newContent string, includeSubsections bool) string {
	target := strings.ToLower(strings.TrimSpace(headerText))
	lines := strings.Split(content, "\n")
	level := 0
	headingLineIdx := -1
	for i, line := range lines {
		if headingLevel(line) == 0 {
			continue
		}
		if strings.ToLower(headingText(line)) == target {
			level = headingLevel(line)
			headingLineIdx = i
			break
		}
	}
	if headingLineIdx < 0 {
		prefix := strings.TrimSpace(headingPrefix)
		if prefix == "" {
			prefix = "###"
		}
		sep := "\n\n"
		if content == "" || strings.HasSuffix(content, "\n") {
			sep = "\n"
		}
		return content + sep + prefix + " " + headerText + "\n\n" + newContent + "\n"
	}
	endIdx := len(lines)
	for j := headingLineIdx + 1; j < len(lines); j++ {
		lvl := headingLevel(lines[j])
		if lvl > 0 && lvl <= level {
			endIdx = j
			break
		}
	}
	var rebuilt []string
	rebuilt = append(rebuilt, lines[:headingLineIdx+1]...)
	rebuilt = append(rebuilt, "")
	for _, l := range strings.Split(newContent, "\n") {
		rebuilt = append(rebuilt, l)
	}
	rebuilt = append(rebuilt, lines[endIdx:]...)
	return strings.Join(rebuilt, "\n")
}

func headingLevel(line string) int {
	trimmed := strings.TrimSpace(line)
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' && n < 6 {
		n++
	}
	if n == 0 || n >= len(trimmed) || trimmed[n] != ' ' {
		return 0
	}
	return n
}

func headingText(line string) string {
	level := headingLevel(line)
	if level == 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimSpace(line)[level:])
}
