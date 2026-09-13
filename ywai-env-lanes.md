# ywai env — parallel lanes (manual dispatch)

Run each block as a subagent from `D:\Yoizen\dev-ai-workflow` (Go module in `ywai/`).
Model for all: `meta/muse-spark-1.3-contributor` · timeout 25 min.

## Status (2026-09-11, verified: build + vet + targeted tests green)

* ✅ Fase 3 — scoped apply (`--profile`, sandbox, restart filter, `uninstall --profile`) — LANDED
* ✅ Fase 5 — web backend `/api/envs` + `Envs.tsx` — LANDED
* ✅ Fase 6 — `eval bench --profile` + `eval compare` — LANDED
* ✅ QA triage — served by central build/vet/targeted-tests run — DONE
* ✅ Fase 4 — preset enforcement (bare short-circuit, scoped filters) — LANDED
* ✅ QA final — full suite 31 ok + CLI smoke clean — DONE
* ✅ `ywai env init` — creates + installs dev/qa/personal from zero (personal bare = 0 files, global byte-identical) — DONE

---

## 5 — Fase 4, `@dev` (preset enforcement)

```
Goal: Enforce presets during profile-scoped apply: the preset stamped in the manifest filters what gets installed. Work item: none.

Context: Repo D:\Yoizen\dev-ai-workflow, Go module ywai/. Package ywai/internal/envprofile EXISTS (Profile{Preset}, Presets()/Preset(name) returning map[string]any with keys: name, description, groups[], skills[], mcp[], default_agent, default_model, deny_bash[], cli_theme, bare, agents[]). Profile-scoped apply EXISTS: applyManaged runs inside envprofile.WithProfileEnv when applyOpts.Profile is set (ywai/cmd/ywai/apply.go), YWAI_PROFILE env marks scope, installAgentProfiles in root.go already skips the canonical dual copy under scope. Opencode2-exclusive, zero V1. You own: ywai/cmd/ywai/root.go (installAgentProfiles filter only), ywai/cmd/ywai/apply.go (preset hook only), and NEW ywai/internal/envprofile/preset_apply.go. Do NOT touch: eval.go, env.go, server.go, control routes, uninstall.go, opencode_restart.go.

Acceptance: (1) NEW preset_apply.go: func PresetSpec(p Profile) (map[string]any, error) (reads Preset(p.Preset), defaults empty); func (bare) IsBare(p) bool. (2) Under profile scope (YWAI_PROFILE set): if bare==true, SKIP agents install, skills copy, MCP/plugin wiring, default_agent/model writes and AGENTS.md entirely (print skip lines; DB/service/pids untouched). (3) Else filter: agent groups = preset groups[] (empty/missing = no filter, keep current default); skills copy = preset skills[] (empty = keep current); MCP/plugin wiring = preset mcp[] (empty = keep current; never uninstall extra servers, only skip installing unlisted ones); default_agent/default_model from preset when non-empty (reuse existing setDefaultAgent/setDefaultModel paths); deny_bash[] from preset appends to the profile's shell-deny rules (read-only wrt global). (4) cli_theme: only note exact wiring line as follow-up comment, do NOT implement. (5) Global (no scope) behavior bit-identical. Code+comments English. No V1.

Verification: no other agents are running — you MAY run `go build ./...` and `go test ./internal/envprofile/` (fast). Do NOT run the full suite (slow; central). Also `gofmt -e` on touched files.

Return format:
```handoff
status: done
did: <per file>
files: <paths>
next: <integration>
verified: <commands + outcomes>
findings: <assumptions>
blockers: <if any>
report: <short>
```
```

---

## 6 — QA final, `@qa` (after Fase 4 lands)

```
Goal: Final verification: full build + full suite + smoke of the env CLI. No code changes. Work item: none.

Context: Repo D:\Yoizen\dev-ai-workflow, Go module ywai/ (run from there). Landed since last green: Fase 4 preset enforcement. No other agents running.

Acceptance: (1) `go build ./...` exit 0. (2) `go test ./... -count=1` full run; triage failures NEW vs PRE-EXISTING (known Windows flakes: plugins statusline/cli.json trio, JobManager timing). (3) Smoke (writes real ~/.ywai, cleans after): `ywai env create qa-smoke --preset qa`, `ywai env list`, `ywai env status qa-smoke`, `ywai env rm qa-smoke --yes`, `ywai env list` — all exit 0 and home left without qa-smoke. Do NOT fix, do NOT commit.

Verification: the commands ARE the verification; paste exit codes, ok/FAIL per package (first 60 lines), smoke transcript.

Return format:
```handoff
status: done
did: <commands run>
files: <inspected>
next: <who fixes what>
verified: <full outcomes>
findings: <triage table>
blockers: <new failures>
report: <short>
```
```
