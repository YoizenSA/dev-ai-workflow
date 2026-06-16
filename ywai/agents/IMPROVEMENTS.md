# Improvements & Changelog

## Resolved

- **PI.dev installer gap**: Added `InstallPi` and `piToolsString` to `internal/agents/agents.go`, wired `case "pi"` in `cmd/ywai/root.go`. PI.dev now receives core agents in its native format (lowercase `tools:`, no `mode:`/`permission:` block).
- **Cross-platform orchestrator prompts**: Replaced hardcoded OpenCode tool names with a capability model + platform adapters table in `core/orchestrator/AGENT.md`. Kanban tracking is now gated behind `ywai-kanban` MCP availability.
- **Frontmatter consistency**: Removed dead `tools:` and `permission:` keys from `finder`/`ask`. All 8 core agents remain `mode: all`. Standardized kanban trailer across all specialists.
