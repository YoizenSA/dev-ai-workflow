# Chat parity plan — ywai vs opencode-manager

Goal: bring the ywai chat (Go proxy + React UI over an OpenCode server) up to the
usability of the reference **opencode-manager** app, for non-technical users.

This is a gap analysis + prioritized roadmap. It is intentionally honest about
scope: opencode-manager is a full TypeScript/Bun product (process supervisor,
auth, multi-repo, voice, notifications). Full parity is multi-week. This plan
slices it so each item ships independently.

Legend: **✅ done** · **◻ missing** · effort **S/M/L**.

---

## 0. Current state (already shipped in ywai)

| Feature | Status |
|---|---|
| Chat send + live SSE streaming (text/reasoning/tool) | ✅ |
| Session list / create | ✅ |
| Message history (typed parts) | ✅ |
| Model picker (in composer) | ✅ |
| Agent picker (in composer) | ✅ |
| Session pins (local store) | ✅ |
| Prompt templates (local store) | ✅ |
| Workspace grouping + switcher + new-chat-in-workspace | ✅ |
| Thinking / tool / subtask collapsible blocks | ✅ |
| Subagent (child session) strip, sync/async labels | ✅ |
| @file mentions, basic slash commands | ✅ (basic) |
| Lightweight markdown rendering | ✅ (basic) |
| Provider list without leaking API keys | ✅ |

Backing endpoints already proxied: `/session*`, `/event`, `/config/providers`,
`/agent`, `/project`, `/session/{id}/children`.

---

## 1. P0 — Core chat correctness & control (highest value)

These are the features whose absence makes the chat feel broken or unsafe for a
non-technical user. Do these first.

### 1.1 Permission requests ◻ **M**
OpenCode pauses and asks the user to approve tool actions (run command, edit
file, etc.). Today ywai has no UI for this, so any session that needs permission
**hangs silently**.
- Events: `permission.updated` on `/event`; reply via `POST /session/{id}/permissions/{permissionID}` (allow/deny/always).
- UI: `PermissionRequestDialog` equivalent — modal or inline card with Allow / Allow always / Deny.
- Backend: proxy the reply endpoint; forward permission events (already flow through `/event`).

### 1.2 Questions from the agent ◻ **M**
Agents can ask the user a free-text question mid-run and block until answered.
- Endpoints: `GET /session/{id}/question`; reply `POST /api/session/{id}/question/{requestID}/reply`, reject `/reject`.
- Events: question events on `/event`.
- UI: `QuestionPrompt` inline + a minimized indicator when scrolled away.

### 1.3 Abort / stop a running turn ◻ **S**
Backend `handleAbort` already exists (`POST /api/chat/abort`) but the UI never
calls it. Add a Stop button that appears while `isStreaming`.
- UI only: swap Send → Stop while streaming, call the existing abort route.

### 1.4 Session management: rename / delete ◻ **S**
Only create exists today.
- Endpoints: `PATCH /session/{id}` (title), `DELETE /session/{id}`.
- UI: `EditSessionTitleDialog`, `DeleteSessionDialog`; row actions in the sidebar.

### 1.5 Send error banner + retry ◻ **S**
Today a failed send shows a flat error string. Reference shows a banner with a
retry action and keeps the drafted text.
- UI only: preserve input on failure, banner with Retry.

### 1.6 Context usage indicator ◻ **S**
Show token/context usage so users know when a session is getting full.
- Data: session `tokens` + model context window (from `/config/providers`), or `GET /session/{id}/context`.
- UI: `ContextUsageIndicator` in the header (a small meter + "compact" hint).

---

## 2. P1 — Rich message rendering (makes replies readable)

### 2.1 File edit / diff rendering ◻ **L**
Tool calls that edit files currently render as raw JSON/text in a `<pre>`. The
reference renders proper diffs.
- Parts/tools: `edit`/`write`/`patch` tool outputs, `PatchPart`; endpoints `GET /session/{id}/diff`.
- UI: `ContentDiffViewer` + `DiffStats` + `FileToolRender` equivalents (added/removed lines, per-file collapsible).

### 2.2 Syntax-highlighted code ◻ **M**
Current markdown code blocks are unstyled monospace. Add highlighting.
- Decision: a highlighter lib (Shiki/Prism) vs. a minimal tokenizer. Offline dep-install risk — confirm before adding a dependency.

### 2.3 Todo / plan display ◻ **M**
Agents emit a todo list; the reference renders live checkable todos.
- Endpoint: `GET /session/{id}/todo`; events update it.
- UI: `SessionTodoDisplay` + `TodoItem`.

### 2.4 Editable user messages / message actions ◻ **M**
Copy, edit-and-resend a previous message, revert.
- Endpoints: `DELETE /session/{id}/message/{messageID}`, `POST /session/{id}/revert`.
- UI: `EditableUserMessage`, `UserMessageActionButtons`.

### 2.5 Richer slash commands ◻ **S**
OpenCode exposes real commands; today ywai hardcodes 4. Pull the command list
and support `POST /session/{id}/command`.
- UI: `CommandSuggestions` fed by the server command list.

### 2.6 Real-time subagent updates ◻ **S**
The subagents strip refreshes on session change / stream end. Make it live by
reacting to child-session events on `/event` (or short polling while running).

---

## 3. P2 — Provider auth & model management

### 3.1 Provider API-key management ◻ **M**
Set/delete provider credentials so users can enable models from the UI.
- Endpoints: `PUT /auth/{providerID}`, `DELETE /auth/{providerID}` (already exist upstream).
- UI: `ApiKeyDialog` + a richer `ModelSelectDialog`.
- Security: keep the no-secret-leak rule; never echo stored keys back.

### 3.2 MCP OAuth flows ◻ **M**
Authenticate MCP servers that need OAuth.
- Endpoints: `/mcp/{name}/auth`, `/provider/{id}/oauth/authorize|callback`.

---

## 4. P3 — Voice, notifications, attachments (nice-to-have)

### 4.1 Text-to-speech ◻ **L** — read assistant replies aloud (`FloatingTTSButton`, `VoiceStatusOverlay`). Needs a TTS backend route.
### 4.2 Speech-to-text ◻ **L** — voice input in the composer (`PromptInput.stt`). Needs an STT backend route.
### 4.3 File upload / attachments ◻ **M** — attach files to a prompt (`FilePartInput`, upload route).
### 4.4 Web push notifications on completion ◻ **S** — ywai already has a push store (`push_*`); wire it to notify when a background/async session finishes.

---

## 5. P4 — Infrastructure / out-of-current-scope

These are big platform pieces from opencode-manager that ywai deliberately does
not replicate today. List them so the decision is explicit, not accidental.

- **OpenCode process supervisor** ◻ **L** — opencode-manager launches/restarts the OpenCode server itself. ywai assumes the user runs `opencode serve`. Add a supervisor only if we want zero-setup.
- **Multi-user auth / login** ◻ **L** — register/login, per-user sessions. Only needed if ywai chat becomes multi-tenant.
- **Multi-repo + SSH key management** ◻ **L** — clone/mirror repos, manage SSH keys. Partially covered by the existing workspace switcher over already-open OpenCode projects.

---

## Suggested delivery order

1. **P0 batch** (permissions, questions, abort, rename/delete, error+retry, context meter) — this is what makes the chat trustworthy end to end. Ship as 2–3 chained PRs.
2. **P1 rendering** (diffs, highlighting, todos, message actions) — the biggest visible quality jump.
3. **P2 provider auth** — unlocks self-service model setup.
4. **P3 voice/attachments/notifications** — pick per user demand.
5. **P4** — only if ywai chat needs to be a standalone product rather than a companion to a running OpenCode server.

## Notes / decisions to confirm before building

- **Syntax highlighting & any new frontend dep**: the build env may be offline; confirm dep install works or stay dependency-free.
- **Permissions default**: auto-allow read-only tools vs. always prompt — product call for the non-technical audience.
- **Auth/supervisor/multi-repo (P4)**: only pursue if the goal is a standalone product; otherwise keep ywai as a thin, polished client over a user-run OpenCode server.
