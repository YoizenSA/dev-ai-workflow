// Shared .adoconfig.toml builder — used by the setup wizard AND the management
// page's collapsible builder. Pure functions, no React.

export interface TomlConfig {
	strategy: "feature-chain" | "stacked";
	baseBranch: string;
	maxLength: number;
	prefix: string;
	requireWorkItem: boolean;
	defaultDraft: boolean;
	allowedTypes: string;
}

export const defaultToml = (): TomlConfig => ({
	strategy: "feature-chain",
	baseBranch: "main",
	maxLength: 10,
	prefix: "feature",
	requireWorkItem: true,
	defaultDraft: true,
	allowedTypes: "feature, fix, hotfix, chore, refactor",
});

export function buildToml(t: TomlConfig): string {
	const types = t.allowedTypes
		.split(",")
		.map((s) => s.trim())
		.filter(Boolean)
		.map((x) => `"${x}"`)
		.join(", ");
	return `# .adoconfig.toml — Project-level ADO conventions

[chain]
strategy = "${t.strategy}"        # "feature-chain" | "stacked"
base_branch = "${t.baseBranch}"
max_length = ${t.maxLength}
prefix = "${t.prefix}"

[branch]
allowed_types = [${types}]
slug_max_length = 40
require_wi_id = true

[pr]
require_work_item = ${t.requireWorkItem}
include_chain_context = true
review_budget = 400
default_draft = ${t.defaultDraft}

[work_item]
auto_transition = false
target_state = "In Dev"
`;
}

// Build a copy-paste shell command that writes .adoconfig.toml into the current
// repo. Heredoc keeps quoting/escaping simple across bash/zsh.
export function buildTomlCommand(toml: string): string {
	return `cat > .adoconfig.toml <<'EOF'
${toml.trimEnd()}\nEOF`;
}
