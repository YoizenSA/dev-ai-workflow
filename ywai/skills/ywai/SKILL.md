---
name: ywai
description: "Run and learn ywai: MCP, model profiles, agent groups, doc tours. Trigger: ywai mcp, ywai profile, /learn-ywai."
---

# ywai

The human does not run these commands. You do.

## When

Two jobs, one skill:

- **Run** — the user wants an MCP on or off, a different model profile, or an agent group installed or removed. Jump to [Commands](#commands).
- **Learn** — the user wants to understand ywai (CLI, agents, skills, workflows) rather than change it. Jump to [Teaching](#teaching).

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

## Teaching

Reach for this when the user wants to learn, not mutate.

- **Docs are the teacher.** Read only `references/docs/` inside this skill. Do not invent commands, flags, or agent names, and do not fetch the website.
- **One page per turn.** Teach that page, then stop. Ask at most one question.
- **Match the user's language.** The docs are Spanish; reply in the user's language.
- **Do not start a `teach/` workspace** (no `MISSION.md` / HTML lessons) unless they ask for a long course.

1. Open [references/curriculum.md](references/curriculum.md).
2. Pick the page: explicit topic, else tour item 1 (or the next unread item if they already started).
3. Read that file from `references/docs/`.
4. Teach from that page only:
   - Lead with the outcome (what they can do after this).
   - Show the happy path (commands / `@agent` examples from the page).
   - One check question so they retrieve, not just nod.
5. Offer the next curriculum item. Do not dump the rest of the site.

If a page is missing or the read fails, say so and teach only what you successfully read.

```bash
/learn-ywai
/learn-ywai agentes
```
