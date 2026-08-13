---
name: ywai
description: >
  Run ywai to enable/disable MCP, switch orchestrator profiles, or
  enable/disable agent groups. Trigger: enable MCP, disable MCP, ywai mcp,
  switch profile, orchestrator profile, inherit profile, ywai profile,
  enable group, disable group, ywai groups.
---

# ywai

The human does not run these commands. You do.

## When

The user wants an MCP on or off, a different orchestrator profile, or an agent group installed or removed.

## Rules

1. **Execute** `ywai …` yourself. Do not paste the command for the user to copy.
2. **List before mutate.** Confirm the id or name exists.
3. After MCP or profile changes, tell them to **restart OpenCode**.
4. Never enable or disable `core`. It stays installed.

## Commands

### MCP

```bash
ywai mcp list
ywai mcp enable <id>
ywai mcp disable <id>
ywai mcp auth <id>
```

`disable` keeps the config. `auth` is OAuth (figma, github, gitlab, or any catalog entry with `AuthType=oauth`).

### Profile

```bash
ywai profile list
ywai profile use <name>
```

`list` marks the active profile with `*`. Shipped names:

| Name | Effect |
|---|---|
| `inherit` | No pinned models. Agents use the session/lead model. |
| `balanced` | Cost/quality mix. DeepSeek v4 pro/flash on the former Grok/Minimax slots. |
| `fast` | Flash model on every agent. |
| `deep` | Top models for design/review. |

### Groups

```bash
ywai groups
ywai groups enable <name>
ywai groups disable <name>
```

Use the names from `ywai groups`. `core` is always on.

## Done when

The command printed success (`MCP … enabled`, `Active profile: …`, `Group … enabled`) or you reported the error verbatim.
