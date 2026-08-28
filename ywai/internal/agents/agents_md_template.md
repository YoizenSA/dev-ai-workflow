# AGENTS.md

ywai-managed instructions. Scope is limited to two concerns: **Engram memory** and **sub-agent strategy**. Persona, SDD, skill catalogs, review hooks, and Graft own their own surfaces — do not add them here.

## Engram Persistent Memory

Memory is what survives compaction and the end of a session. Nothing else does.

Save (`mem_save`) when you learn something the next session would otherwise rediscover: a decision and its tradeoffs, a bug's root cause, a convention, a gotcha, a user constraint. Do it as it happens, not at the end — detail decays fast, and the save that gets skipped is always the one you postponed.

Search before assuming you have no context. `mem_context` first (fast, recent), then `mem_search` for keywords, then `mem_get_observation` for the full untruncated content of a hit. Do this whenever the user references past work in any language, and when a request touches something that may already have been solved.

### Save format

- **title**: verb + what, searchable — "Fixed N+1 query in UserList"
- **type**: bugfix | decision | architecture | discovery | pattern | config | preference
- **scope**: `project` (default) | `personal`
- **topic_key**: use a stable key (`architecture/auth-model`) when a topic will evolve, so updates upsert instead of fragmenting. Unsure → `mem_suggest_topic_key`. Know the ID → `mem_update`.
- **capture_prompt**: `false` only for automated artifacts (caches, registry output, generated state). Omit otherwise.
- **content**: what was done · why (request, bug, performance) · where (paths) · what surprised you

Distinct topics must never overwrite each other. A memory returned as `needs_review` is stale context, not a fact — surface it to the user and verify against current evidence before relying on it. Never mark one reviewed without explicit confirmation.

### Session close

Before saying you are done, call `mem_session_summary` covering: goal, user preferences or constraints discovered, discoveries, what was accomplished, what remains, and the relevant files with what changed in each. Skipping this starts the next session blind.

### After compaction

On a compaction message, `mem_session_summary` with the compacted content **first** — that is the only moment the pre-compaction work can still be persisted — then `mem_context` to recover earlier context, then continue.

## Sub-Agents

### One launch per task

Keep a session-scoped list of the `(phase, task-fingerprint)` pairs you have launched, where the fingerprint is the phase plus the key artifacts named in the instruction. Never launch a pair twice. Duplicate launches race on the same files and produce "File X has been modified since it was last read" — the failure looks like a tooling bug and is not.

### Skills: match by trigger, load by id

OpenCode injects `<available_skills>` into every prompt and exposes the `skill` tool, which loads a skill by its id. Match the task against those descriptions and call `skill` directly — there is no registry to resolve and no path to pass.

When delegating, name the skill ids the sub-agent should load in its brief. It receives its own `<available_skills>` and loads them the same way.

### Context protocol

A sub-agent starts with no memory, so you control what it knows. Search Engram yourself and pass the relevant context into the prompt; the sub-agent should not go looking for handoff context on its own. For prior work, pass topic keys or observation IDs and let it fetch the full content — not a dump.

The sub-agent holds the detail, so it saves before returning: tell it to `mem_save` significant discoveries, decisions, and fixes with `project: '{project}'`. After it returns, that detail is gone.

### Language

The active persona governs conversation with the user, never the artifacts. Generated code, comments, docs, tests, and commit messages default to English unless the user or the existing project clearly requires otherwise. Forward this when delegating, so persona voice does not leak into the work.

Write that English to ASD-STE100 (Simplified Technical English): use one word for one meaning, keep the active voice, and use simple tenses. Give each sentence one topic. Keep a procedure sentence to 20 words and a descriptive sentence to 25. Keep the articles. Do not stack nouns and do not chain gerunds. These rules apply to the artifacts, never to conversation with the user.
