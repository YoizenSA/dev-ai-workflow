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

// OpenCode v2 model-facing delegation tools (Code Mode / opencode2).
// Permission action remains `subagent`; the callable tool is `delegate`.
const (
	OpenCodeDelegateToolHint      = "Delegate with OpenCode v2 `delegate`. Use `mode: \"sync\"` when the next step needs the result, `mode: \"async\"` (default) for independent work. Supervise with `delegation_peek` / `delegation_steer` / `delegation_stop` / `delegation_read`.\n"
	OpenCodeDelegateSemanticGuard = "Semantic guard: **delegate** means the native `delegate` tool launching a configured sub-agent. Permission action `subagent` gates who may be launched. Delegation is that tool call; scripts, Python, and Bash are execution.\n\n"
)

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

// AgentDelegation is the per-agent entry: the subagent map + optional rule/triggers
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

// ApplyDelegations writes the per-agent delegation graph (v2 subagent rules)
// into opencode.json, renders the rules+triggers into each agent's markdown
// prompt body, AND persists the full doc as agentsDir/delegations.json so the
// Orchestrator UI can read/write rules as structured JSON (never parsing .md).
// It is idempotent: re-running with the same doc leaves files unchanged.
//
// configPath is the opencode.json/.jsonc path. agentsDir is where the .md files
// live (e.g. ~/.config/opencode/agents).
//
// Only agents whose markdown is already installed (<agentsDir>/<name>.md exists)
// are touched. This matters because a default `ywai install` ships just the core
// group, but delegations.json lists agents from other groups (qa-automation,
// migration-*, social-refactor). Writing a delegation task map for an agent with
// no installed .md would create a stub entry in opencode.json with no
// description/prompt, which opencode rejects ("Expected string | undefined, got
// null description"). The markdown application paths already skip missing files;
// this filter extends the same guard to the opencode.json write and the sidecar.
func ApplyDelegations(configPath, agentsDir string, doc *DelegationsDoc) error {
	if doc == nil || len(doc.Agents) == 0 {
		return nil
	}

	doc = filterToInstalledAgents(agentsDir, doc)
	if len(doc.Agents) == 0 {
		return nil
	}

	if err := applyTaskMaps(configPath, doc); err != nil {
		return err
	}
	// opencode enforces agent permissions from the markdown frontmatter, not
	// the opencode.json agent.<name> entry (that is only the UI's canonical
	// store). The delegation task graph must live in the .md to actually gate
	// who each agent may delegate to.
	if err := applyTaskMapsToMarkdown(agentsDir, doc); err != nil {
		return err
	}
	if err := applyRulesToMarkdown(agentsDir, doc); err != nil {
		return err
	}
	return persistDelegationsSidecar(agentsDir, doc)
}

// filterToInstalledAgents returns a copy of doc restricted to agents whose
// markdown file exists in agentsDir. The original doc is returned unchanged
// when every agent is installed (the common case), so re-applying delegations
// after a full --all-groups install stays a zero-allocation pass-through.
func filterToInstalledAgents(agentsDir string, doc *DelegationsDoc) *DelegationsDoc {
	if len(doc.Agents) == 0 {
		return doc
	}
	installed := make(map[string]bool, len(doc.Agents))
	missing := 0
	for name := range doc.Agents {
		if _, err := os.Stat(filepath.Join(agentsDir, name+".md")); err == nil {
			installed[name] = true
		} else {
			missing++
		}
	}
	if missing == 0 {
		return doc
	}
	filtered := &DelegationsDoc{Agents: make(map[string]AgentDelegation, len(installed))}
	filtered.Defaults = doc.Defaults
	for name, ad := range doc.Agents {
		if installed[name] {
			filtered.Agents[name] = ad
		}
	}
	return filtered
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

// applyTaskMaps writes the per-agent delegation graph from the doc into
// opencode.json as v2 `permissions` rules (action subagent) under
// agent.<name>. Legacy v1 `permission.task` maps are removed when present.
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

	agents := map[string]any{}
	if raw, ok := root["agents"].(map[string]any); ok {
		agents = raw
	}
	if leftover, ok := root["agent"].(map[string]any); ok {
		for k, v := range leftover {
			if _, exists := agents[k]; !exists {
				agents[k] = v
			}
		}
		delete(root, "agent")
	}
	root["agents"] = agents

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
		// Fold any leftover v1 permission map into v2 rules, then overlay the
		// subagent whitelist. Generated JSON must not keep permission/task/bash.
		var rules []PermissionRule
		if perm, ok := entry["permission"].(map[string]any); ok {
			internal := map[string]string{}
			for k, raw := range perm {
				if s, ok := raw.(string); ok && s != "" {
					internal[k] = s
				}
			}
			rules = RulesFromPermissionMap(name, internal)
			delete(entry, "permission")
		}
		rules = ReplaceSubagentRules(rules, ad.Task)
		entry["permissions"] = RulesToJSONShape(rules)
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

// applyTaskMapsToMarkdown injects the per-agent subagent delegation rules
// into each agent's .md frontmatter. This is the location opencode actually
// enforces; the opencode.json copy (applyTaskMaps) is only what the UI reads.
func applyTaskMapsToMarkdown(agentsDir string, doc *DelegationsDoc) error {
	applied := 0
	for name, ad := range doc.Agents {
		if len(ad.Task) == 0 {
			continue
		}
		path := filepath.Join(agentsDir, name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue // agent markdown not installed yet; skip
		}
		updated, ok := injectTaskPermission(string(data), ad.Task)
		if !ok || updated == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			fmt.Printf("  Warning: failed to write task map to %s: %v\n", path, err)
			continue
		}
		applied++
	}
	if applied > 0 {
		fmt.Printf("  Applied delegation subagent rules into %d agent prompts\n", applied)
	}
	return nil
}

// InjectTaskPermission is the exported entry point for writing a delegation
// task map into an agent's markdown frontmatter. opencode merges markdown
// agents on top of opencode.json (markdown wins), so the .md is the only
// reliably-enforced location for subagent rules.
func InjectTaskPermission(content string, task map[string]string) (string, bool) {
	return injectTaskPermission(content, task)
}

// ReadTaskPermission extracts the delegation map (subagent rules) from an
// agent's markdown frontmatter. v2 rules are read from the `permissions:`
// array (action subagent); legacy v1 files fall back to the nested
// permission.task block. A missing block yields an empty map. ok is false
// only when there is no frontmatter at all.
func ReadTaskPermission(content string) (map[string]string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, false
	}
	fmEnd := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fmEnd = i
			break
		}
	}
	if fmEnd < 0 {
		return nil, false
	}
	fm := strings.Join(lines[1:fmEnd], "\n")

	result := map[string]string{}
	if rules, ok := ParsePermissionRulesYAML(fm); ok {
		for _, r := range rules {
			if r.Action == "subagent" {
				result[r.Resource] = r.Effect
			}
		}
		return result, true
	}
	return readLegacyTaskBlock(fm), true
}

// readLegacyTaskBlock parses the v1 permission.task nested map.
func readLegacyTaskBlock(fm string) map[string]string {
	result := map[string]string{}
	fmLines := strings.Split(fm, "\n")
	permIdx := -1
	for i, line := range fmLines {
		if strings.TrimSpace(line) == "permission:" {
			permIdx = i
			break
		}
	}
	if permIdx < 0 {
		return result
	}
	permIndent := leadingSpaces(fmLines[permIdx])
	childIndent := permIndent + 2
	for i := permIdx + 1; i < len(fmLines); i++ {
		if strings.TrimSpace(fmLines[i]) == "" {
			continue
		}
		ind := leadingSpaces(fmLines[i])
		if ind <= permIndent {
			break
		}
		if ind == childIndent && permKeyName(fmLines[i]) == "task" {
			val := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(fmLines[i]), permKeyRaw(fmLines[i])))
			val = strings.TrimSpace(strings.TrimPrefix(val, ":"))
			if val != "" {
				result["*"] = strings.Trim(val, `"'`)
				return result
			}
			for j := i + 1; j < len(fmLines); j++ {
				if strings.TrimSpace(fmLines[j]) == "" {
					continue
				}
				if leadingSpaces(fmLines[j]) <= childIndent {
					break
				}
				k := permKeyName(fmLines[j])
				v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(fmLines[j]), permKeyRaw(fmLines[j])))
				v = strings.TrimSpace(strings.TrimPrefix(v, ":"))
				if k != "" {
					result[k] = strings.Trim(v, `"'`)
				}
			}
			return result
		}
	}
	return result
}

// injectTaskPermission swaps the subagent (delegation) rules inside the
// frontmatter `permissions:` array, leaving every other rule untouched. When
// the file still carries a legacy v1 `permission:` block, it is removed: the
// v2 rules replace it wholesale so the two cannot disagree. Returns
// (content, false) when there is no frontmatter to patch.
func injectTaskPermission(content string, task map[string]string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	fmEnd := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fmEnd = i
			break
		}
	}
	if fmEnd < 0 {
		return content, false
	}

	fmLines := lines[1:fmEnd]
	rules, hasRules := ParsePermissionRulesYAML(strings.Join(fmLines, "\n"))
	if !hasRules {
		// Legacy v1 file: convert its permission map so sibling tool rules
		// survive; the block itself is replaced by the rules array.
		rules = LegacyPermissionBlockToRules(strings.Join(fmLines, "\n"))
		if rules == nil {
			// No permission data at all — inventing a block would strip the
			// file's implicit posture, so leave it alone.
			return content, false
		}
	}
	rules = ReplaceSubagentRules(rules, task)

	// Keep every frontmatter line except the v2 permissions block, any legacy
	// v1 permission block, and `group:`. The permission blocks are re-emitted
	// from the rules below. `group:` is dropped because opencode v2 accepts a
	// fixed agent key set; an unknown key sends the whole file down the legacy
	// v1 decode path, which drops the v2 permissions array. Group membership
	// lives in GroupSidecarFile. Stripping it here also cleans files installed
	// before that move, since this rewriter otherwise preserves them forever.
	var kept []string
	for i := 0; i < len(fmLines); i++ {
		trimmed := strings.TrimSpace(fmLines[i])
		if strings.HasPrefix(trimmed, "group:") {
			continue
		}
		if trimmed == "permissions:" || trimmed == "permission:" ||
			strings.HasPrefix(trimmed, "permission:") {
			for i+1 < len(fmLines) && (isIndented(fmLines[i+1]) || strings.TrimSpace(fmLines[i+1]) == "") {
				i++
			}
			continue
		}
		kept = append(kept, fmLines[i])
	}
	kept = append(kept, RenderPermissionRulesYAML(rules)...)

	rebuilt := append([]string{"---"}, kept...)
	rebuilt = append(rebuilt, "---")
	if rest := lines[fmEnd+1:]; len(rest) > 0 {
		rebuilt = append(rebuilt, rest...)
	}
	return strings.Join(rebuilt, "\n"), true
}

// isIndented reports whether the line starts with whitespace.
func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func leadingSpaces(s string) int {
	n := 0
	for _, c := range s {
		if c == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

func permKeyName(line string) string {
	t := strings.TrimSpace(line)
	idx := strings.Index(t, ":")
	if idx < 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(t[:idx]), `"'`)
}

// permKeyRaw returns the raw (quoted) key token of a "key: value" line.
func permKeyRaw(line string) string {
	t := strings.TrimSpace(line)
	idx := strings.Index(t, ":")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(t[:idx])
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
		_ = WriteAgentBackup(path, data)
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
		b.WriteString("\n")
		b.WriteString(OpenCodeDelegateToolHint)
	}

	if len(triggers) > 0 {
		b.WriteString("\n#### Mandatory Delegation Triggers\n\n")
		b.WriteString("These gates are **non-skippable hard gates**, not recommendations.\n\n")
		b.WriteString(OpenCodeDelegateSemanticGuard)
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
// avoid an import cycle with the configapi package's copy.)
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
