# Workflow Prompt Sections — Choose, Seed & Edit (Design & Plan)

## Context

Prompt "sections" are shared `.md` files under `ywai/agents/sections/`
(`handoff.md`, `handoff-qa.md`, `context-gathering.md`). Core agents inherit
them by declaring `sections: [handoff, ...]` in frontmatter; `AppendSections`
(`ywai/internal/agents/agents.go`) reads each file and appends it to the prompt.

**Proven need for per-node choice.** The `qa-automation` family already varies
sections by role:

| Agent | sections |
|---|---|
| qa-analyst / qa-ask / qa-dev / qa-finder / qa-reviewer | `[handoff-qa, context-gathering]` |
| qa-orchestrator | `[handoff]` |

So the selection is **per node/role**, not per workflow — even one family mixes
`handoff` and `handoff-qa`.

**Current state.** The workflow exporter (`internal/workflows/export.go`)
hardcodes `[]string{"handoff"}` for every sub-agent and injects nothing into the
orchestrator. There is **no UI** to view, choose, or edit sections; the only UI
touchpoint is the read-only handoff callout in `NodeDetail.tsx`.

## Goals

1. Let each workflow node choose which sections it injects (not a fixed
   `handoff`).
2. Surface the available sections and each node's selection in the canvas UI.
3. (Later) Let users create/edit their own sections without corrupting the
   shared, core-agent-facing ones.

## The core architectural decision (read before building)

Sections are **global and shared** between core agents and workflows. Any
"edit sections from the canvas" feature must answer: *edit what?*

- **Edit the shared file** → silently changes the handoff of every core agent.
  Dangerous side effect. **Rejected as a default.**
- **Edit a per-scope copy** → workflows/users get their own editable copies;
  shared seed files stay immutable. Preserves the single-source-of-truth for
  core agents.

Decision: **seed sections are read-only shared defaults; user sections are a
separate editable scope.** This mirrors the existing seed pattern in
`internal/config/data.go` (`SeedWorkflowsFromEmbedded`, `DataAgentsDir`,
`ShouldSeedData`) that already seeds bundled defaults and preserves user edits.

## Delivery in three phases (ship Phase 1 first)

Phase 1 solves the real qa-automation case with zero data-model risk. Phases 2–3
are the "edit/create" vision and are optional/deferred.

---

## Phase 1 — Choose existing sections per node

### 1.1 Data model

- Go: add to `NodeData` (`ywai/internal/workflows/model.go`):
  ```go
  // Sections lists the shared prompt sections (from agents/sections/) this node
  // injects into its system prompt on export. Empty means the export default.
  Sections []string `json:"sections,omitempty"`
  ```
- TS: add `sections?: string[]` to `WorkflowNodeData`
  (`internal/control/web/src/api/types.ts`).
- Backwards compatible: `omitempty` + optional, so existing workflow JSON loads
  unchanged.

### 1.2 Exporter reads the field (with role-aware defaults)

File: `internal/workflows/export.go`

- Sub-agent (`renderSubAgentMarkdown`): replace the hardcoded literal with
  ```go
  sections := n.Data.Sections
  if len(sections) == 0 {
      sections = []string{"handoff"} // default for sub-agents
  }
  prompt = agents.AppendSections(prompt, sections, config.AgentsSourceDir())
  ```
  Do this once on the shared `prompt` string BEFORE the target branch so both
  opencode and claude-code paths inherit it (removes today's duplicated
  injection in the two branches).
- Orchestrator (`renderOrchestratorMarkdown`): honor `START`/orchestrator node
  `Sections`, default **empty** (unchanged behavior — orchestrator injects
  nothing unless the user opts in). This fixes the earlier over-simplification:
  `qa-orchestrator` proves an orchestrator MAY carry `handoff` when it reports
  upward.

### 1.3 List-sections API

File: `internal/control/workflows.go`

- `GET /api/workflows/sections` → returns the available section names + content:
  ```go
  // reads AgentsSourceDir()/sections/*.md, returns [{name, content}]
  ```
- Keep the existing `GET /api/workflows/handoff-contract` or fold it into the
  list endpoint (the inspector disclosure can read `content` from the list).
- Whitelist to `*.md` files in that one dir; no arbitrary path reads.

### 1.4 UI — per-node selector

File: `internal/control/web/src/components/workflows/NodeDetail.tsx`

- In `SubAgentFields` (and the orchestrator fields), replace the static
  `HandoffContractSection` with a `SectionsField`:
  - A `MultiSelect` (component already exists: `MultiSelect.tsx`) populated from
    `GET /api/workflows/sections`.
  - `selected = node.data.sections ?? []`; on change
    `update(node, { sections })`.
  - Placeholder / help text: "Empty = default (`handoff` for sub-agents)."
  - Keep the collapsible viewer to preview a selected section's `.md` content
    (reuse the fetched `content` from the list endpoint — single source of
    truth, no hardcoded copy).
- Client: replace `workflowApi.handoffContract()` with
  `workflowApi.listSections()` in `api/client.ts`, cached like the skill/mcp
  catalogs.

### 1.5 UI — canvas badge

File: `internal/control/web/src/components/workflows/nodes.tsx`

- Replace the fixed `📋 handoff` badge with the node's actual sections:
  - Sub-agent with no explicit sections → `📋 handoff` (shows the default).
  - Explicit sections → show them (e.g. `📋 handoff-qa +1`), full list in the
    `title` tooltip.

### 1.6 Tests

- Extend `internal/workflows/export_test.go`:
  - Node with `Sections: ["handoff-qa"]` → agent md contains the handoff-qa
    marker, NOT the `handoff.md`-only marker.
  - Node with empty `Sections` → still gets `handoff` (default preserved).
  - Orchestrator with `Sections: ["handoff"]` → contains it; empty → contains
    none.
  - Use a **hermetic temp fixture** for `AgentsSourceDir` (write a tiny
    `sections/handoff.md` + `sections/handoff-qa.md` with distinct markers) so
    the test does not depend on the real repo section text.
- Frontend: a `NodeDetail` test asserting the MultiSelect renders options from a
  mocked `listSections` and writes `node.data.sections` on change.

### Phase 1 size

Backend small (1 model field, 1 exporter refactor, 1 list endpoint), frontend
medium (selector + badge + client). ~7 files. No editing, no new storage scope.

---

## Phase 2 — Create & edit user sections (opt-in scope)

Only if a concrete need to *edit* (not just choose) appears.

- **Storage**: user sections live in `DataAgentsDir()/sections/` (the seeded,
  user-writable copy), separate from the read-only source `sections/`. Seed
  bundled defaults there via the existing `data.go` seed path; preserve user
  edits on re-seed (same guarantee `SeedWorkflowsFrom` already gives workflows).
- **Resolution order** in `AppendSections`' caller: user dir first, then source
  dir. (Requires letting the exporter/agent loader consult both dirs — a small
  change to how `sourceDir` is resolved for sections specifically.)
- **API**: `POST/PUT/DELETE /api/workflows/sections/{name}` writing only under
  `DataAgentsDir()/sections/`. Seed defaults are read-only (reject writes to a
  name that shadows a bundled seed, or copy-on-write into the user dir).
- **UI**: the section preview becomes an editor for user sections; seed sections
  stay read-only with a "Duplicate to edit" action.

## Phase 3 — Sections library screen (full vision)

A dedicated "Prompt Sections" management screen (like the Skills screen):
list / create / edit / delete user sections globally, with clear read-only
badges on seed sections. Reuses the Phase 2 storage + API. Deferred until
choosing + per-workflow editing prove insufficient.

## Non-goals (for now)

- Editing shared/core-agent sections from the canvas (the trap — rejected).
- Versioning/history of section edits beyond what git/seed-preserve already give.
- Per-node inline ad-hoc sections (use the Task prompt field for one-offs).

## Verification / self-check

- [ ] `go test ./internal/workflows/...` — new selection + default + orchestrator
      cases pass with a hermetic fixture.
- [ ] Export a QA-style workflow: a worker node set to `handoff-qa` produces the
      QA handoff; a node left empty still produces `handoff`.
- [ ] Canvas badges reflect each node's real sections; tooltip lists them.
- [ ] Inspector MultiSelect lists all `sections/*.md`; preview shows real file
      content (change the file → preview changes: proves single source of truth).
- [ ] Existing workflow JSON (no `sections` field) exports identically to today.

## Migration / compatibility

- Field is additive and optional → old workflows unaffected.
- Default preserves current behavior (`handoff` for sub-agents, none for
  orchestrator), so Phase 1 is a pure superset of the shipped implementation.
