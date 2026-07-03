# Workflow Handoff Injection — Design & Implementation Plan

## Problem

Core agents (`ywai/agents/core/*`) assemble their system prompt by declaring
`sections: [handoff, ...]` in frontmatter; `extractSections` reads that list and
`appendSections` (`ywai/internal/agents/agents.go:216`) appends the shared
`ywai/agents/sections/<name>.md` file to the prompt. This is how every core
sub-agent inherits the handoff contract ("report back to @orchestrator" with
Status / Did / Artifacts / Next / Notes).

Workflow-exported agents do **not** get this. The exporter
(`ywai/internal/workflows/export.go`) builds each agent's prompt straight from
node data (`AgentDefinition` / `Prompt`) and never runs the section-injection
step. So a workflow's sub-agents ship without any handoff protocol, and the
person building the workflow on the canvas has no way to know a handoff contract
should exist.

## Goals

1. **Runtime**: workflow-exported sub-agents carry the same `handoff` section as
   core sub-agents, for both export targets (opencode + claude-code).
2. **Visibility**: someone viewing/editing the workflow on the canvas can *see*
   that each sub-agent auto-receives the handoff contract — without opening the
   generated `.md`.

## Design decisions

- **Reuse, don't reinvent.** The injection mechanism already exists and is
  target-agnostic (both render paths take a `body`/`prompt` string). We inject
  into that string before rendering — one point per agent covers both dialects.
- **Hardcoded default now, canvas override later.** Phase 1 always injects
  `handoff` into sub-agents. We do **not** add a `NodeData.sections` field yet:
  a model field with no UI to set it is dead weight (YAGNI). When a real
  per-node control is built, the switch to a hybrid default+override is a
  one-line change (`if len(n.Data.Sections) > 0 { … }`).
- **Orchestrator does not get `handoff`.** `handoff.md` says "report back to
  @orchestrator" — it is the contract for *emitters* (sub-agents). The
  orchestrator is the *receiver*; injecting handoff there is incoherent. If the
  orchestrator needs guidance, it is a different section (how to *consume*
  handoffs), out of scope here.
- **Single source of truth for the section text.** The UI must not hardcode a
  copy of the handoff protocol. It reads the same `sections/handoff.md` the
  exporter injects, via a small read-only API. Otherwise the canvas copy and the
  exported copy drift.

## Part 1 — Export injection (backend)

### 1.1 Expose the injection helper

`appendSections` is currently private. Export a thin wrapper (keep the private
one to avoid touching core callers):

- File: `ywai/internal/agents/agents.go`
- Add:
  ```go
  // AppendSections is the exported entry point so other packages (e.g. the
  // workflow exporter) can reuse the same section-injection the core agent
  // loader uses. sourceDir is the agents root (see config.AgentsSourceDir()).
  func AppendSections(prompt string, sections []string, sourceDir string) string {
      return appendSections(prompt, sections, sourceDir)
  }
  ```

### 1.2 Inject into sub-agent bodies

- File: `ywai/internal/workflows/export.go`
- Function: `renderSubAgentMarkdown` (line ~303). After resolving `prompt`
  (lines 316–319) and before building the profile / claude markdown:
  ```go
  prompt = agents.AppendSections(prompt, subAgentSections, config.AgentsSourceDir())
  ```
- Add a package constant near the other export defaults:
  ```go
  // subAgentSections are the shared prompt sections every workflow sub-agent
  // inherits on export, mirroring the core agents' `sections:` frontmatter.
  var subAgentSections = []string{"handoff"}
  ```
- Injection happens on the `body` string, so it flows through **both**
  `renderClaudeAgentMarkdown` and `BuildOpenCodeMarkdown` unchanged. The
  existing `## Task` appending (lines 323–325 / 345–347) still runs after — the
  handoff section lands in the identity body, the task stays last. Verify the
  ordering reads well; if the task must come before the handoff, inject after
  the `## Task` concatenation instead.
- `config` is already imported in `export.go`.

### 1.3 Do NOT inject into the orchestrator

Leave `renderOrchestratorMarkdown` untouched. Document why with a one-line
comment referencing this decision.

### 1.4 Backend tests

- File: `ywai/internal/workflows/export_test.go` (extend existing).
- Assert a workflow with one sub-agent node produces agent markdown that
  contains a distinctive line from `sections/handoff.md` (e.g. the
  `**Status**:` token).
- Assert the orchestrator markdown does **not** contain it.
- Cover both targets (opencode + claude-code) via the existing
  `NewExporterWithDirsForTarget` helper. The test must point `AgentsSourceDir`
  at a fixture or the real repo `ywai/agents` — prefer a temp fixture with a
  minimal `sections/handoff.md` so the test is hermetic and does not break if
  the real handoff copy changes.

## Part 2 — Canvas visibility (frontend)

The user building the workflow must understand the handoff contract exists. Two
complementary surfaces, cheapest first:

### 2.1 Inspector callout (primary)

- File: `ywai/internal/control/web/src/components/workflows/NodeDetail.tsx`
- In `SubAgentFields` (line ~369), add a read-only informational block after the
  task prompt field. It states: *"On export, this agent automatically receives
  the shared **handoff** contract — it must end each run with Status / Did /
  Artifacts / Next / Notes so the orchestrator can route the next step."*
- Include an expandable "View handoff contract" disclosure that renders the
  actual `sections/handoff.md` content (see 2.3 for the API). Collapsed by
  default to avoid clutter.
- Style with the existing `field-help` / callout classes in
  `WorkflowEditor.css`; no new design system work.

### 2.2 Canvas badge (secondary, at-a-glance)

- File: `ywai/internal/control/web/src/components/workflows/nodes.tsx`
- Render a small badge/icon (e.g. a handshake or document glyph from
  `lucide-react`, already a dependency) on `subAgent` node cards, with a
  `title`/tooltip "Includes handoff contract". This makes the contract visible
  without opening the inspector.
- Keep it static in Phase 1 (every sub-agent has it). When per-node override
  lands, the badge becomes conditional.

### 2.3 Read-only section API (single source of truth)

The inspector disclosure must show the real file, not a hardcoded copy.

- Backend: add `GET /api/workflows/sections/{name}` (or reuse an existing
  workflow-scoped handler in `ywai/internal/control/workflows.go`) that returns
  the contents of `sections/<name>.md` from `config.AgentsSourceDir()`.
  Whitelist the name to the known sections (`handoff`, `handoff-qa`,
  `context-gathering`) — no arbitrary path reads.
- Frontend: add `workflowApi.getSection(name)` in `api/client.ts`, fetched once
  and cached like the existing skill/mcp catalogs in `NodeDetail.tsx`.

## UX summary

| Surface | What the user sees | When |
|---|---|---|
| Canvas node badge | "handoff" glyph on every sub-agent card | Always visible |
| Inspector callout | Plain-language note + expandable full contract | On node select |
| Export preview modal | The injected section inside the generated `.md` | On export (already works once Part 1 lands) |

## Out of scope (future)

- Per-node `NodeData.sections` field + multi-select UI to choose sections
  (upgrade to hybrid default+override).
- Orchestrator "consume handoff" section.
- Editing handoff contract text from the canvas.

## Self-check / verification

- [ ] `go test ./internal/workflows/...` — new injection tests pass (both targets).
- [ ] Export a real workflow, open a sub-agent `.md`, confirm the handoff block
      is present and the orchestrator `.md` has none.
- [ ] Open the workflow in the Studio: sub-agent nodes show the badge; the
      inspector callout renders and the disclosure loads the real
      `handoff.md` text via the API.
- [ ] Change `sections/handoff.md`, reload the inspector — the disclosure
      reflects the change (proves single source of truth, no drift).

## Rough size

- Part 1 (backend + tests): small — 1 exported wrapper, 1 injection line, 1
  const, ~2 tests. ~3 files.
- Part 2 (frontend + API): medium — 1 endpoint, 1 api client method, inspector
  callout + disclosure, canvas badge. ~5 files.

Chained delivery recommended: PR #1 = Part 1 (runtime correctness, independently
valuable), PR #2 = Part 2 (visibility) on top.
