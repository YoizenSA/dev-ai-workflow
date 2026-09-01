package agents

// permissions_v2.go — opencode v2 permission rules.
//
// OpenCode v2 replaced the v1 `permission:` map (tool-glob → allow/deny/ask)
// with an ordered array of rules under `permissions:`:
//
//	permissions:
//	  - action: shell
//	    resource: "*"
//	    effect: deny
//	  - action: shell
//	    resource: "git diff *"
//	    effect: allow
//
// Semantics that drive everything in this file:
//   - The LAST matching rule wins, so broad rules go first and exceptions after.
//   - `bash` → action `shell`, `task` → action `subagent`, `write` merges into
//     `edit` (edit/write/patch share the edit action).
//   - MCP tools are actions named `<server>_<tool>`; wildcards allowed.
//   - Unmatched → "ask" against base defaults that broadly allow tools, so a
//     deny-by-default whitelist must start with explicit deny rules.
//
// ywai keeps its internal vocabulary (read/edit/write/bash/task + coarse
// buckets) as the source of truth for profiles and the UI; this file is the
// single translation layer to the v2 rule format that opencode enforces.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// PermissionRule is one opencode v2 permission rule.
type PermissionRule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

// Valid effects per the v2 rule schema.
const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
	EffectAsk   = "ask"
)

// v2NativeActions are the core v2 permission actions ywai manages explicitly,
// in canonical emission order. A profile that omits one gets a deny rule, so
// only the whitelisted surface is exposed (v1 behavior preserved).
var v2NativeActions = []string{
	"read", "edit", "glob", "grep", "websearch", "webfetch",
	"question", "skill", "subagent", "shell",
}

// v1KeyToV2Action maps ywai's internal permission keys to v2 actions. Keys not
// listed (MCP globs like engram_*/graft_*, plugin tools) are valid v2 actions
// as-is. v2 synonyms (shell, subagent) normalize back onto the internal names.
var v1KeyToV2Action = map[string]string{
	"bash": "shell",
	"task": "subagent",
}

// v2ActionToV1Key is the reading direction: v2 action → internal key.
var v2ActionToV1Key = map[string]string{
	"shell":    "bash",
	"subagent": "task",
}

// droppedV2Keys had a v1 permission key but no v2 action exists (the tool was
// removed or cannot be permission-gated): lsp, ast_grep, code_search,
// todowrite, list, doom_loop. Silently skipped on emission.
var droppedV2Keys = map[string]bool{
	"lsp": true, "ast_grep": true, "code_search": true,
	"todowrite": true, "list": true, "doom_loop": true,
}

// NormalizePermissionKey maps v2 action names onto ywai's internal vocabulary
// (shell→bash, subagent→task) so hand-edited files and UI payloads written in
// v2 terms still hit the right bucket. Identity for everything else.
func NormalizePermissionKey(key string) string {
	if v1, ok := v2ActionToV1Key[key]; ok {
		return v1
	}
	return key
}

// V2ActionForInternalKey returns the v2 action for an internal key, or "" when
// the key has no v2 enforcement point.
func V2ActionForInternalKey(key string) string {
	if droppedV2Keys[key] {
		return ""
	}
	if a, ok := v1KeyToV2Action[key]; ok {
		return a
	}
	return key
}

// writeScopeResources preserves the v1 edit/write split for agents whose
// write permission was scoped to specific paths. v2 merges write into edit,
// so "edit: deny + write: allow" becomes "deny edit *" followed by scoped
// allow rules — the paths those agents are documented to write.
var writeScopeResources = map[string][]string{
	"planning":      {".plans", ".plans/*"},
	"planner-draft": {".plans", ".plans/*"},
	// test-author wrote via the ADO MCP, not files: its write:allow was
	// vestigial, so the merge resolves to the stricter edit value.
}

// RulesFromPermissionMap converts ywai's internal permission map into an
// ordered slice of v2 rules. baseName is the flat agent id (write-scope and
// no-commit lookups). Ordering is semantic — last match wins:
// broad rules first, scoped allows after, guardrail denies last.
func RulesFromPermissionMap(baseName string, perms map[string]string) []PermissionRule {
	perms = ExpandPermissionBuckets(perms)
	var rules []PermissionRule
	emit := func(action, resource, effect string) {
		rules = append(rules, PermissionRule{Action: action, Resource: resource, Effect: effect})
	}

	seen := map[string]bool{}
	// val resolves an internal key, falling back to its v2 synonym so maps
	// built from v2 payloads (UI, hand-edited files) hit the same bucket.
	val := func(key string) (string, bool) {
		v, ok := perms[key]
		if !ok {
			if v2, isV2 := v1KeyToV2Action[key]; isV2 {
				v, ok = perms[v2]
			}
		}
		if !ok || v == "" {
			return "", false
		}
		return v, true
	}

	// 1. Broad native rules in canonical order; omitted natives default to
	// deny so unlisted tools stay hidden (v1 whitelist semantics).
	for _, action := range v2NativeActions {
		key := action
		if v1, ok := v2ActionToV1Key[action]; ok {
			key = v1
		}
		v, ok := val(key)
		if !ok {
			v = EffectDeny
		}
		if v == "verify" {
			v = EffectDeny // verify expands below; the base posture is deny-all
		}
		if action == "edit" {
			// write merges into edit. When the two disagree the agent had a
			// scoped write surface: emit the stricter broad value first, then
			// the scoped allows after it (last match wins). Without a known
			// scope the stricter value wins — never widen.
			seen["edit"] = true
			seen["write"] = true
			emit("edit", "*", v)
			// Scoped allows must come after the broad rule (last match wins).
			if w, hasWrite := val("write"); hasWrite && w != v {
				if scope, hasScope := writeScopeResources[baseName]; hasScope {
					for _, res := range scope {
						emit("edit", res, w)
					}
				}
			}
			continue
		}
		if action == "shell" {
			seen["bash"] = true
			emit("shell", "*", v)
			continue
		}
		seen[key] = true
		emit(action, "*", v)
	}

	// 2. Remaining keys (buckets expanded, MCP globs, plugin tools), sorted for
	// deterministic output. These are broad rules; shell's pattern-level
	// exceptions come after in pass 3.
	var remaining []string
	for k := range perms {
		a := V2ActionForInternalKey(k)
		if a == "" || seen[k] {
			continue
		}
		remaining = append(remaining, k)
	}
	sort.Strings(remaining)
	for _, k := range remaining {
		emit(V2ActionForInternalKey(k), "*", perms[k])
	}

	// 3. Shell exceptions: the verify allowlist, then guardrail denies last so
	// they override any allow above them.
	if b, ok := val("bash"); ok && b != EffectDeny {
		if b == "verify" {
			for _, p := range verifyBashAllowPatterns {
				emit("shell", p, EffectAllow)
			}
		}
		for _, p := range falseGreenBashPatterns {
			emit("shell", p, EffectDeny)
		}
		if noCommitAgents[baseName] && b != "verify" {
			for _, p := range noCommitBashDenyPatterns {
				emit("shell", p, EffectDeny)
			}
		}
	}

	return rules
}

// ApplySkillAllowlist replaces a broad skill:* allow with deny-all plus
// per-id allows. Empty ids leave rules unchanged. Last match wins, so
// the specific allows must come after the deny.
func ApplySkillAllowlist(rules []PermissionRule, ids []string) []PermissionRule {
	if len(ids) == 0 {
		return rules
	}
	out := make([]PermissionRule, 0, len(rules)+len(ids)+1)
	for _, r := range rules {
		if r.Action == "skill" {
			continue
		}
		out = append(out, r)
	}
	out = append(out, PermissionRule{Action: "skill", Resource: "*", Effect: EffectDeny})
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, PermissionRule{Action: "skill", Resource: id, Effect: EffectAllow})
	}
	return out
}

// SubagentRulesFromTaskMap converts a v1 delegation task map (agent id →
// effect) into ordered subagent rules: the "*" catch-all first, specific ids
// after, so an explicit entry always overrides the catch-all.
func SubagentRulesFromTaskMap(task map[string]string) []PermissionRule {
	if len(task) == 0 {
		return []PermissionRule{{Action: "subagent", Resource: "*", Effect: EffectAllow}}
	}
	var rules []PermissionRule
	if v, ok := task["*"]; ok {
		rules = append(rules, PermissionRule{Action: "subagent", Resource: "*", Effect: v})
	}
	ids := make([]string, 0, len(task))
	for id := range task {
		if id != "*" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		rules = append(rules, PermissionRule{Action: "subagent", Resource: id, Effect: task[id]})
	}
	return rules
}

// ReplaceSubagentRules swaps every subagent-action rule in rules for the ones
// derived from task. Non-subagent rules keep their relative order.
func ReplaceSubagentRules(rules []PermissionRule, task map[string]string) []PermissionRule {
	out := make([]PermissionRule, 0, len(rules))
	inserted := false
	for _, r := range rules {
		if r.Action == "subagent" {
			if !inserted {
				out = append(out, SubagentRulesFromTaskMap(task)...)
				inserted = true
			}
			continue
		}
		out = append(out, r)
	}
	if !inserted {
		out = append(out, SubagentRulesFromTaskMap(task)...)
	}
	return out
}

// InternalMapFromRules reduces v2 rules back onto ywai's internal vocabulary.
// Only broad (resource "*") rules map to simple keys; scoped rules (e.g. the
// .plans write surface, shell patterns) are dropped — they are re-derived at
// render time and have no flat representation. subagent rules belong to the
// delegation graph and are surfaced via the task-permissions reader instead.
func InternalMapFromRules(rules []PermissionRule) map[string]string {
	out := map[string]string{}
	for _, r := range rules {
		if r.Resource != "*" {
			continue
		}
		switch r.Action {
		case "subagent":
			continue
		case "edit":
			out["edit"] = r.Effect
			out["write"] = r.Effect // v2 cannot split them; report one surface
		case "shell":
			out["bash"] = r.Effect
		default:
			key := NormalizePermissionKey(r.Action)
			if _, exists := out[key]; !exists {
				out[key] = r.Effect
			}
		}
	}
	return out
}

// V1PermissionFromRules is the opencode.json v1 `permission` map. OpenCode v1
// rejects a v2 `permissions` array on agent entries. Scoped non-task rules
// (shell patterns) have no v1 representation and are dropped.
func V1PermissionFromRules(rules []PermissionRule) map[string]any {
	out := map[string]any{}
	for k, v := range InternalMapFromRules(rules) {
		out[k] = v
	}
	task := map[string]string{}
	for _, r := range rules {
		if r.Action != "subagent" || r.Effect == "" {
			continue
		}
		res := r.Resource
		if res == "" {
			res = "*"
		}
		task[res] = r.Effect
	}
	if len(task) > 0 {
		out["task"] = task
	}
	return out
}

// RewriteOpenCodeJSONV1Permissions converts leftover v2 `permissions` arrays
// on agent.<name> into v1 `permission` maps. OpenCode v1 rejects the array
// (`agent.orchestrator.permissions`). Returns how many agents were rewritten.
func RewriteOpenCodeJSONV1Permissions(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return 0, err
	}
	agentRaw, ok := root["agent"]
	if !ok {
		return 0, nil
	}
	var agentsMap map[string]json.RawMessage
	if err := json.Unmarshal(agentRaw, &agentsMap); err != nil {
		return 0, err
	}
	changed := 0
	for name, raw := range agentsMap {
		var cfg map[string]json.RawMessage
		if json.Unmarshal(raw, &cfg) != nil {
			continue
		}
		rulesRaw, ok := cfg["permissions"]
		if !ok {
			continue
		}
		var rules []PermissionRule
		if json.Unmarshal(rulesRaw, &rules) != nil {
			continue
		}
		perm, err := json.Marshal(V1PermissionFromRules(rules))
		if err != nil {
			continue
		}
		cfg["permission"] = perm
		delete(cfg, "permissions")
		rewritten, err := json.Marshal(cfg)
		if err != nil {
			continue
		}
		agentsMap[name] = rewritten
		changed++
	}
	if changed == 0 {
		return 0, nil
	}
	agentsJSON, err := json.Marshal(agentsMap)
	if err != nil {
		return changed, err
	}
	root["agent"] = agentsJSON
	pretty, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return changed, err
	}
	pretty = append(pretty, '\n')
	return changed, os.WriteFile(path, pretty, 0o644)
}

// RenderPermissionRulesYAML renders rules as the frontmatter block under
// "permissions:" (indented list of - action/resource/effect maps).
func RenderPermissionRulesYAML(rules []PermissionRule) []string {
	lines := []string{"permissions:"}
	for _, r := range rules {
		lines = append(lines,
			fmt.Sprintf("  - action: %s", yamlScalar(r.Action)),
			fmt.Sprintf("    resource: %s", yamlScalar(r.Resource)),
			fmt.Sprintf("    effect: %s", yamlScalar(r.Effect)),
		)
	}
	return lines
}

// yamlScalar quotes a YAML scalar whenever a bare one would not survive a
// strict parser: globs like "*" or "* -u", and any text carrying a structural
// character. Descriptions go through here too — they end in "Trigger: ...", and
// a plain scalar may not contain ": ". Emitting one bare broke opencode's v2
// agent schema, which then fell back to the legacy v1 decode and turned the
// whole file, frontmatter included, into the system prompt.
func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, "*:#&!|>',[]{}%`@\"\\\n\r\t") ||
		strings.HasPrefix(s, "?") || strings.HasPrefix(s, "!") ||
		strings.TrimSpace(s) != s {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// ParsePermissionRulesYAML extracts the rules from a frontmatter
// "permissions:" block. ok is false when the block is absent. Handles the
// canonical rendering above plus reasonable hand-edited variants (quoted or
// unquoted scalars, extra blank lines) — it is a line scanner, not a YAML
// parser, matching the rest of this package's frontmatter handling.
func ParsePermissionRulesYAML(fm string) ([]PermissionRule, bool) {
	lines := strings.Split(fm, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "permissions:" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	var rules []PermissionRule
	var cur *PermissionRule
	flush := func() {
		if cur != nil && cur.Action != "" && cur.Effect != "" {
			if cur.Resource == "" {
				cur.Resource = "*"
			}
			rules = append(rules, *cur)
		}
		cur = nil
	}
	for i := start; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// List item start: "  - action: shell" (dash at any shallow indent).
		if strings.HasPrefix(trimmed, "- ") {
			flush()
			cur = &PermissionRule{}
			k, v, ok := splitYAMLKV(trimmed[2:])
			if ok && k == "action" {
				cur.Action = v
			}
			continue
		}
		// Indented property of the current item; a non-indented key: value
		// means the block ended.
		if line != trimmed || strings.HasPrefix(trimmed, " ") {
			k, v, ok := splitYAMLKV(trimmed)
			if !ok || cur == nil {
				continue
			}
			switch k {
			case "action":
				cur.Action = v
			case "resource":
				cur.Resource = v
			case "effect":
				cur.Effect = v
			}
			continue
		}
		break
	}
	flush()
	return rules, true
}

// splitYAMLKV splits "key: value" with optional quotes on the value.
func splitYAMLKV(s string) (string, string, bool) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(s[:idx])
	val := strings.TrimSpace(s[idx+1:])
	val = strings.Trim(val, `"'`)
	return key, val, key != ""
}

// LegacyPermissionBlockToRules converts a v1 frontmatter `permission:` map
// into broad v2 rules so a legacy file's tool posture survives a delegation
// injection. Scalar children map to resource-"*" rules; a nested bash block
// contributes its broad ("*") value as the shell rule and its per-command
// patterns as additional shell rules.
func LegacyPermissionBlockToRules(fm string) []PermissionRule {
	lines := strings.Split(fm, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "permission:" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	var rules []PermissionRule
	childIndent := -1
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		ind := leadingSpaces(line)
		if childIndent == -1 {
			childIndent = ind
		}
		if ind < childIndent {
			break // block ended
		}
		k, v, ok := splitYAMLKV(strings.TrimSpace(line))
		if !ok {
			continue
		}
		if ind > childIndent {
			// Nested bash pattern (4-space under "bash:"): carried over as a
			// shell rule so deny patterns survive the conversion.
			continue
		}
		if v == "" {
			continue // nested header (e.g. "bash:") — children handled below
		}
		action := V2ActionForInternalKey(k)
		if action == "" {
			continue
		}
		rules = append(rules, PermissionRule{Action: action, Resource: "*", Effect: v})
	}
	return rules
}

// RulesToJSONShape converts rules to the []map[string]any shape written into
// opencode.json agent entries (agent.<name>.permissions).
func RulesToJSONShape(rules []PermissionRule) []any {
	out := make([]any, 0, len(rules))
	for _, r := range rules {
		out = append(out, map[string]any{
			"action":   r.Action,
			"resource": r.Resource,
			"effect":   r.Effect,
		})
	}
	return out
}
