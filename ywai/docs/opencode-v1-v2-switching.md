# OpenCode v1 vs OpenCode 2 (v2) Switching Architecture

## Overview
This document specifies how `ywai` handles switching between OpenCode v1 (`opencode`) and OpenCode 2 (`opencode2`), ensuring full compatibility across configuration schemas, MCP definitions, plugins, agent frontmatter, and server APIs.

---

## 1. User Configuration & Web UI
- **User Config**: `~/.ywai/config.yaml` stores `opencode_version: "v1" | "v2" | ""` (empty means autodetect, preferring `opencode2`).
- **CLI Commands**:
  - `ywai config set opencode_version v1`
  - `ywai config set opencode_version v2`
  - `ywai config get opencode_version`
- **Environment Override**:
  - `YWAI_OPENCODE=v1` or `YWAI_OPENCODE=v2` (wins over stored configuration for one-off runs and test pipelines).
- **Web UI (Control Server)**:
  - Accessible via `Settings` -> `General` -> `OpenCode version` selector.
  - Automatically loads and saves via `/api/config/user`.

---

## 2. Binary Resolution
- Handled centrally by `ywai/internal/agent/agent.go`:
  - `FindOpenCode()`: resolves the active binary path and name.
  - `OpenCodeBinaryName()`: returns `"opencode"` or `"opencode2"`.
  - `OpenCodeIsV2()`: boolean helper indicating whether OpenCode 2 is active.
- Consumers:
  - `host.BinaryName(OpenCode)` delegates to `agent.OpenCodeBinaryName()`.
  - `local_client.go` resolves candidates based on `agent.OpenCodeBinaryName()`.

---

## 3. MCP Server Persistence
Defined in `ywai/internal/mcp/agent_config.go` and consumed by `control/mcp_store.go` and `plugins/mcp.go`:

| Feature | OpenCode v1 | OpenCode 2 (v2) |
| :--- | :--- | :--- |
| **Location** | Flat under `mcp.<id>` | Nested under `mcp.servers.<id>` |
| **Activation** | Explicit `"enabled": true \| false` | Default enabled; uses `"disabled": true` |
| **Local Config** | `{"type": "local", "command": [...], "env": {...}}` | `{"type": "local", "command": [...], "environment": {...}}` |
| **Remote Config** | `{"type": "remote", "url": "..."}` | `{"type": "remote", "url": "..."}` |

### Reading & Writing
- `mcp.CollectOpenCodeServers(section)` reads from either layout.
- `mcp.WriteOpenCodeMCP(existing, servers)` writes the target layout based on `OpenCodeIsV2()`.

---

## 4. Plugins
- **Config Key**:
  - v1: `"plugin": [...]`
  - v2: `"plugins": [...]`
- **Incompatible v1 Plugins**:
  - `sub-agent-statusline` and `background-agents` are automatically skipped when `OpenCodeIsV2()` is true because v2 changed plugin hooks and dropped `parentID`.
- **TUI/CLI Plugins**:
  - v1: `~/.config/opencode/tui.json` with `"plugin": [...]`
  - v2: `~/.config/opencode/cli.json` with `"plugins": [...]`

---

## 5. Agent Frontmatter & Delegation Rules
- **Markdown Frontmatter (`~/.config/opencode/agents/<name>.md`)**:
  - v1 uses nested YAML map:
    ```yaml
    permission:
      edit: allow
      bash:
        "*": allow
        git * commit*: deny
    ```
  - v2 uses ordered rule array (last match wins):
    ```yaml
    permissions:
      - action: shell
        resource: "*"
        effect: allow
      - action: shell
        resource: "git * commit*"
        effect: deny
    ```
- **Delegation Injection (`delegations.go`)**:
  - v1 updates `permission.task` map.
  - v2 replaces `subagent` rules in `permissions:` array via `injectSubagentRules`.

---

## 6. Server Client API Endpoints
- In `ywai/internal/opencode/server_client.go`, `apiPath()` routes requests:
  - v1: `/agent`, `/provider`
  - v2: `/api/agent`, `/api/provider` (avoids SPA HTML shell returned on bare routes).
