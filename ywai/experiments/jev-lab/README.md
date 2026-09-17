# jev-lab — experimental environment for the Jev gate layer

Status: **experiment** — nothing here ships, seeds, or loads outside this directory.

Source plan: `PLAN.md` in this directory (copied from
`C:\Users\Nahuel\Downloads\plan-jev-opencode2-refinado.md`, v2 refined).
The plan is the source of truth for thresholds, question shapes, and the §6.7
open-question list. This README only records what changes now that the plan
lands inside ywai as an experiment.

Reference implementation: https://github.com/devagrawal09/jev-review (MIT).
Spike A questions and the SDK call shape are extracted from its
`src/review/judgments.ts`; Fase 1 should vendor its `src/domain/` + `src/review/`
instead of rewriting (`PLAN.md` §7: "copiar y adaptar, no reescribir de memoria").

## Ground rules

1. The plugin lives in `.opencode/plugins/jev-gate/` — OpenCode 2 auto-discovers
   it **only** when a session starts from this directory. Never add it to the
   global `~/.config/opencode/` config or any ywai-seeded config.
2. Jev decides, never talks: it returns typed signals (Choice / Score / Noul
   with probability + confidence). It never writes patches and never relaxes a
   permission. Gates only harden (allow → ask), never loosen.
3. Fail open on optimization (Pick), fail closed with a notice on what claims to
   be Jev (Review, Find).
4. Every Fase 0 assumption about the OC2 plugin API is written as a question and
   answered in `FINDINGS.md` before building on it (plan §6.7).

## What ywai changes about the plan

| Plan idea | ywai adjustment |
|---|---|
| Pick motivation | Stronger here: a ywai-seeded opencode carries ~100+ MCP tool schemas (engram 22, graft 6, browser 44, chrome-devtools 29). Spike B measures the real ceiling. |
| Find | Compared against graft (structural graph), not raw grep. Question: does semantic search find what the graph does not? |
| Review | Compared against the `reviewer` agent + `code-review` skill. Question: fewer false positives per useful finding? |
| Route | Compared against the orchestrator triage prompt (solo/thin/full). Question: cheaper and as accurate? |
| Distribution (if promoted) | v1 = tools behind the ywai MCP daemon (review/find/route, low risk). v1.1 = OC2 plugin for Pick + permission gates (high risk). See "Promotion path" below. |

## Layout

```
.opencode/plugins/jev-gate/   spike B: hello-world plugin, logs only
fixtures/                     adversarial patches (screen + eval seeds)
scripts/spike-screen.mjs      spike A: one screen call against a fixture
FINDINGS.md                   written answers to plan §6.7 (fill in Fase 0)
PLAN.md                       the source plan, copied in-repo
package.json                  @typesafe-ai/sdk for spike A
```

## Fase 0 — how to run it

Spike A (Jev signal):

```bash
bun install   # once, pulls @typesafe-ai/sdk
# key from https://console.typesafe.ai/settings/keys
TYPESAFE_API_KEY=... bun scripts/spike-screen.mjs fixtures/02-removed-auth-check.patch
```

Done when: response arrives < 3 s, usage + confidence visible, and the removed
auth check scores security >= 0.70. Then repeat with
`03-manipulator-comment.patch`: the comment must NOT lower that score.

Spike B (OC2 plugin API):

```bash
cd .opencode/plugins/jev-gate && bun install
cd ../../.. && opencode2   # open a session from THIS directory, do anything
# then read .spike/log.jsonl
```

Done when `FINDINGS.md` answers all of plan §6.7, including the schema-size
ceiling for Pick with a real ywai config.

## Decision gate after Fase 0

- Review/Find signals beat or complement the existing ywai stack on fixtures
  → continue with plan Fases 1-2 (core + review tool) in this lab.
- Schema ceiling too small to pay for Pick → Pick stays off permanently; plan
  says so; Fases 4-5 shrink to gates only.
- Nothing beats the existing stack → stop here. The experiment cost one week,
  ywai cost nothing.

## Promotion path (how ywai would install it)

ywai already ships TypeScript OpenCode plugins this way (`background-agents` is
the working reference); jev-gate would copy its pattern, not invent one:

1. **Source + bundle**: new `ywai/plugins/jev-gate/` mirroring
   `ywai/plugins/background-agents/` — TS source compiled to ONE self-contained
   file `dist/jev-gate.js` (`bun build`; `@typesafe-ai/sdk` bundles in, no
   runtime `node_modules`). The lab dir here is never installed; the promoted
   source is a copy, pruned to what survived the experiment.
2. **Go plumbing** (small, mirrors existing):
   - `internal/config/data.go`: `JevGateBundleName` + `JevGateBundlePath()`
     with the same 3-step resolution (source `dist/` → seeded data dir →
     embedded FS).
   - `internal/plugins/jev_gate.go`: `InstallJevGate(configPath)` delegating to
     `installVendorJS` with `ManifestEntry{Bundle: "jev-gate"}` + entry in the
     `vendorBundles` map.
3. **Opt-in flag**: `ywai install --jev-gate` (default off), same pattern as
   `--ponytail`. Experiments are never default-on.
4. **Config safety**: `installVendorPluginV2` already strips stale entries from
   the config `plugins` array and installs by auto-discovery dir only — that
   answers PLAN §6.7.5 (no double load) with machinery that already works.
5. **Options/keys**: no config seeding needed; the plugin reads
   `TYPESAFE_API_KEY` + env/defaults. Fewer moving parts than the
   `opencode.jsonc` options block the plan sketched.
6. **Skill**: `ywai/skills/jev-gate/SKILL.md` with the `.ywai-extra` marker —
   the standard extra-skill seeding, added only when the plugin ships.
7. **Lifecycle**: `ywai update` re-runs the same path; `prepare-embedded.sh`
   rebuilds the bundle into the binary for production builds.

**Update**: the installer plumbing (bundle, Go wiring, `--jev-gate` TUI option)
was built up front by explicit user decision, with the SPIKE plugin as content —
so the experiment can collect evidence on real projects through the normal
installer. The spike changes nothing; the real plugin content lands through the
same path only after the decision gate below.

Nothing else here starts before the lab passes its decision gate; the Go wiring
itself is done and this section now documents what exists.
