# ywai Agents: Improvements Roadmap

Analysis of current implementation against production agent harness best practices (Anthropic, OpenAI, LangChain).

## Context

Based on "The Anatomy of an Agent Harness" — the harness is the complete software infrastructure wrapping an LLM: orchestration loop, tools, memory, context management, state persistence, error handling, and guardrails.

**Current state**: ywai provides agent definitions (AGENT.md, tools.json) but the execution loop runs in the host (OpenCode, Claude Code, Cursor, etc.). ywai is a "configuration harness" that injects into the host's harness.

## Current Strengths

- Multi-agent with clear roles (orchestrator + 5 specialists)
- Standard handoff format for communication
- Fan-out for parallel delegation
- Tool scoping (minimal tools per agent)
- Harness thin (configuration, no custom loop)
- ENGRAM integration (persistent memory MCP server) — installed via `ywai install`

## Resolved

- **Delegation tool mismatch (fixed)**: The orchestrator instructed the model to *poll* `delegation_list()`, which contradicted the `background-agents` plugin's rules ("NEVER poll — you are notified via `<task-notification>`"). Resolved by adopting a **hybrid** model: synchronous `task` for the sequential spine, asynchronous `delegate` (notification-based, no polling) for fan-out. `Task` is now mapped in `agents.go` and allowed in the orchestrator's `tools.json`.

## Gaps and Improvements

### 1. Error Handling (HIGH PRIORITY)

**Current state**: No error classification. Errors are handled by the host.

**Gap**: A 10-step process with 99% per-step success has only ~90.4% end-to-end success. Errors compound fast.

**Proposed implementation**:

#### A. Add error classification to standard handoff

Update handoff format to include optional error fields:

```
**Status**: done | blocked | needs-decision
**Error type**: transient | llm-recoverable | user-fixable | unexpected (optional)
**Error details**: <...>
**Did**: <summary>
**Artifacts**: <...>
**Next suggested**: @dev | @qa | @reviewer | @devops | close
**Notes/risks**: <...>
```

**Error types** (based on LangGraph):
- **Transient**: timeout, rate limit, network glitch — retry with backoff
- **LLM-recoverable**: tool call failed due to bad arguments, model can self-correct — re-delegate with correction
- **User-fixable**: missing permissions, design decision, ambiguity — use `question` tool
- **Unexpected**: harness bug, unknown error — bubble up to user

#### B. Add error handling logic to orchestrator

Add section "Error Handling" in `orchestrator/AGENT.md`:

```markdown
## Error Handling

When a subagent returns `Status: blocked` with an error:

1. **Classify the error** based on the handoff's `Error type`:
   - **Transient**: retry the same delegation up to 2 times with exponential backoff
   - **LLM-recoverable**: re-delegate with clarification/correction (add context to the brief)
   - **User-fixable**: use the `question` tool to ask the user for input or decision
   - **Unexpected**: bubble up to the user with full error details

2. **Track retries** with `todowrite` — mark each retry attempt

3. **After 2 failed retries**, escalate to user-fixable or unexpected

4. **Persist learned errors** with `mem_save` (ENGRAM) — title, type, What/Why/Where/Learned
```

#### C. Update subagent handoffs

Add error fields to handoffs in:
- `architect/AGENT.md`
- `dev/AGENT.md`
- `qa/AGENT.md`
- `reviewer/AGENT.md`
- `devops/AGENT.md`

**Impact**: Reduces error compounding, enables self-recovery, provides learnings for future sessions.

---

### 2. Verification Loops (HIGH PRIORITY)

**Current state**: Manual — delegate to `@qa`/`@reviewer` but no automatic verification.

**Gap**: Anthropic reports verification loops improve quality by 2-3x. Current approach is reactive, not proactive.

**Proposed implementation**:

#### A. Add "Gather-Act-Verify" to orchestrator flow

Update delivery flow in `orchestrator/AGENT.md` to make verification explicit:

```markdown
## Delivery Flow (state machine)

```
GOAL
  └─ PLAN        → delegate @architect (design / plan, ADR if needed)
  └─ TDD?        → ask the user (question tool): "Do we use TDD for this?"
       ├─ yes →  TEST(red)  → delegate @qa   (write failing tests first)
       │         IMPLEMENT   → delegate @dev  (make tests pass, green)
       │         VERIFY      → delegate @qa  (run tests, confirm green)
       └─ no  →  IMPLEMENT   → delegate @dev  (build feature)
                 VERIFY      → delegate @qa  (run tests, add coverage)
  └─ REVIEW      → delegate @reviewer
       ├─ changes requested → back to @dev (fix) then @reviewer again
       └─ approved          → continue
  └─ DEPLOY?     → delegate @devops (CI/CD, container, deploy) when relevant
  └─ CLOSE       → summarize delivered work, artifacts, and follow-ups
```

Each action phase (IMPLEMENT, DEPLOY) is followed by a VERIFY phase.
```

#### B. Add verification types

Add section "Verification Strategies" in `orchestrator/AGENT.md`:

```markdown
## Verification Strategies

Use the appropriate verification type for each phase:

- **Computational verification**: tests, linters, type checkers (deterministic ground truth)
- **Inferential verification**: LLM-as-judge (catches semantic issues, adds latency)
- **Visual verification**: screenshots via Playwright for UI tasks

For code changes: delegate `@qa` to run tests/linters and report pass/fail.
For infra changes: delegate `@devops` to validate deployment.
For design changes: delegate `@reviewer` for semantic review.
```

**Impact**: Proactive quality assurance, catches failures before they compound, aligns with "Gather-Act-Verify" pattern.

---

### 3. Context Management (MEDIUM PRIORITY)

**Current state**: Delegated to host. No compaction, observation masking, or JIT retrieval.

**Gap**: Context rot degrades performance 30%+ when key content falls in mid-window positions (Chroma research, Stanford's "Lost in the Middle").

**Proposed implementation**:

#### A. Sub-agent delegation summaries

Update "Delegation Brief Format" in `orchestrator/AGENT.md` to specify summary length:

```markdown
## Delegation Brief Format

Every delegation (tool or `@mention`) must include:

```
**Goal**: <the one-line objective for this subagent>
**Context**: <relevant files, decisions, prior handoffs>
**Acceptance criteria**: <what "done" means, observable>
**Expected artifacts**: <code / tests / ADR / pipeline / report>
**Constraints**: <stack, patterns, scope limits>
**Output format**: Return a condensed summary (1,000-2,000 tokens max) in the handoff
```

Subagents should return condensed summaries, not full transcripts.
```

#### B. Add context compaction guidance

Add section "Context Management" in `orchestrator/AGENT.md`:

```markdown
## Context Management

To avoid context rot:

- **Compaction**: When approaching context limits, summarize conversation history. Preserve architectural decisions and unresolved bugs; discard redundant tool outputs.
- **Observation masking**: Keep tool calls visible but hide old tool outputs (use `todowrite` to track state instead of relying on conversation history).
- **JIT retrieval**: Use lightweight identifiers (file paths, function names) and load data dynamically via `Read`/`Grep` rather than loading full files.

Goal: find the smallest possible set of high-signal tokens that maximize likelihood of the desired outcome.
```

**Impact**: Reduces context window pressure, maintains performance over long sessions, aligns with Anthropic's context engineering guide.

---

### 4. ENGRAM Integration (MEDIUM PRIORITY)

**Current state**: ENGRAM is installed via `ywai install` as a gentle-ai component, but not explicitly used in agent prompts.

**Gap**: Agents don't leverage ENGRAM's persistent memory (mem_save, mem_search, mem_context, mem_timeline).

**Proposed implementation**:

#### A. Map ENGRAM MCP tools in agents.go

Add `engramToolMap` to `internal/agents/agents.go`:

```go
// engramToolMap maps internal tool names to Engram MCP tool names.
var engramToolMap = map[string]string{
	"MemSave":           "mem_save",
	"MemSearch":         "mem_search",
	"MemContext":        "mem_context",
	"MemSessionStart":   "mem_session_start",
	"MemSessionEnd":     "mem_session_end",
	"MemSessionSummary": "mem_session_summary",
	"MemTimeline":       "mem_timeline",
}
```

#### B. Add ENGRAM tools to orchestrator

Update `orchestrator/permissions.json` to include ENGRAM tools:

```json
{
  "read": "allow",
  "glob": "allow",
  "grep": "allow",
  "websearch": "allow",
  "code_search": "allow",
  "task": "allow",
  "delegate": "allow",
  "delegation_list": "allow",
  "delegation_read": "allow",
  "question": "allow",
  "todowrite": "allow",
  "edit": "deny",
  "write": "deny",
  "bash": "deny"
}
```

#### C. Add memory usage to orchestrator

Add section "Memory (ENGRAM)" in `orchestrator/AGENT.md`:

```markdown
## Memory (ENGRAM)

ENGRAM provides persistent cross-session memory via MCP. Use it to:

- **Session tracking**: Call `mem_session_start` at the beginning of a goal, `mem_session_end` when closing.
- **Save handoffs**: After each subagent handoff, call `mem_save` with:
  - **Title**: "Architect design for checkout feature" / "QA regression test for #1234"
  - **Type**: "design" / "implementation" / "test" / "review" / "deployment" / "error"
  - **What**: summary of what was done
  - **Why**: rationale / acceptance criteria
  - **Where**: affected files / modules
  - **Learned**: lessons learned (especially for errors)
- **Search context**: Before delegating, call `mem_search` to retrieve relevant past work.
- **Timeline**: Use `mem_timeline` to see project history.

Memory is a hint — always verify against actual state before acting.
```

**Impact**: Provides continuity across sessions, enables learning from past work, aligns with production harness memory patterns.

---

### 5. State Persistence (LOW PRIORITY)

**Current state**: Handoffs provide state, but no checkpointing or time-travel debugging.

**Gap**: No resume after interruptions, no progress tracking across sessions.

**Proposed implementation**:

#### A. Add progress file to orchestrator

Add section "State Persistence" in `orchestrator/AGENT.md`:

```markdown
## State Persistence

Maintain a `PROGRESS.md` file in the project root to track goal progress:

```markdown
# Goal: [goal description]

## Status
- Current phase: PLAN / IMPLEMENT / REVIEW / DEPLOY / CLOSE
- Started: [timestamp]
- Last updated: [timestamp]

## Completed
- [x] PLAN: architect handoff received
- [ ] IMPLEMENT: dev in progress
- [ ] REVIEW: pending
- [ ] DEPLOY: pending

## Handoffs
- architect: [summary] (link to ENGRAM memory)
- dev: [summary] (link to ENGRAM memory)
```

Update this file after each handoff. Use it to resume after interruptions.
```

**Impact**: Enables resume after context window exhaustion, provides visibility for users, aligns with LangGraph checkpointing.

---

## Implementation Priority

| Improvement | Priority | Effort | Impact |
|-------------|----------|--------|--------|
| Error Handling | HIGH | Medium | Reduces error compounding |
| Verification Loops | HIGH | Low | 2-3x quality improvement |
| ENGRAM Integration | MEDIUM | Medium | Cross-session continuity |
| Context Management | MEDIUM | Low | Performance stability |
| State Persistence | LOW | Low | Resume capability |

## Notes

- All improvements are additive — they don't require breaking changes to existing agents.
- ENGRAM is already installed via `ywai install`, so integration is about usage, not installation.
- Error handling and verification loops provide the highest ROI for production-grade behavior.
- As models improve, harness complexity should decrease (co-evolution principle). These improvements are designed to be thin scaffolding that can be removed when models internalize the capabilities.
