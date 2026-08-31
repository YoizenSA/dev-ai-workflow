// toolCatalog defines the tools and delegation targets a node can expose.
// Used by the multi-select pickers in NodeDetail.

// All tools/buckets a node can use. Buckets (mcp, memory, delegate) expand to
// opencode-native wildcards at export time via ExpandPermissionBuckets, so the
// user picks a friendly name and the export handles the rest.
export interface ToolOption {
	value: string
	label: string
	group: 'core' | 'mcp' | 'meta'
}

export const TOOL_OPTIONS: ToolOption[] = [
	// Core tools (values use ywai's internal vocabulary; the exporter maps
	// them onto opencode v2 actions — bash→shell, task→subagent, write→edit).
	{ value: 'read', label: 'Read', group: 'core' },
	{ value: 'edit', label: 'Edit', group: 'core' },
	{ value: 'write', label: 'Write', group: 'core' },
	{ value: 'bash', label: 'Bash (shell)', group: 'core' },
	{ value: 'glob', label: 'Glob', group: 'core' },
	{ value: 'grep', label: 'Grep', group: 'core' },
	{ value: 'websearch', label: 'Web Search', group: 'core' },
	{ value: 'webfetch', label: 'Web Fetch', group: 'core' },
	// Meta / workflow tools
	{ value: 'delegate', label: 'Delegate (async sub-agent)', group: 'meta' },
	{ value: 'question', label: 'Question (ask user)', group: 'meta' },
	{ value: 'skill', label: 'Skill', group: 'meta' },
	// MCP buckets (expand to wildcards at export)
	{ value: 'mcp', label: 'MCP (every configured server)', group: 'mcp' },
	{ value: 'graft', label: 'Graft (repo graph)', group: 'mcp' },
	{ value: 'memory', label: 'Memory (engram)', group: 'mcp' },
]

// Tools a coordinator (orchestrator) gets by default. Read-only + delegation.
export const DEFAULT_ORCHESTRATOR_TOOLS = ['read', 'glob', 'grep', 'delegate', 'question', 'skill', 'mcp']

// Tools a developer/implementer gets by default. Full dev access.
export const DEFAULT_DEV_TOOLS = ['read', 'edit', 'write', 'bash', 'glob', 'grep', 'graft', 'skill', 'mcp']

// Known external agents that sub-agents commonly delegate to (from
// delegations.json). These are agents that exist in the opencode ecosystem
// but aren't part of this workflow — the user can still delegate to them.
export const EXTERNAL_AGENTS = ['finder', 'memory', 'ask', 'architect', 'dev', 'qa', 'reviewer', 'devops']

// Parse a CSV string into a Set of selected values.
export function csvToSet(csv: string | undefined): Set<string> {
	if (!csv) return new Set()
	return new Set(
		csv
			.split(',')
			.map((s) => s.trim().toLowerCase())
			.filter(Boolean),
	)
}

// Convert a Set back to a sorted CSV string.
export function setToCsv(set: Set<string>): string {
	return [...set].sort().join(',')
}
