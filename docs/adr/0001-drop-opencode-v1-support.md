# ADR-0001: Drop OpenCode v1 support, require OpenCode 2

## Status

Accepted (2026-09-11)

## Context

ywai supported both OpenCode v1 (`opencode`) and v2 (`opencode2`) through a
compatibility layer selected by `agent.OpenCodeIsV2()` — 22 branch sites across
14 files: dual permission schemas, dual MCP/plugin/provider config shapes, a
v1 evals dialect, a UI flavor selector (`YWAI_OPENCODE` env, `opencode_version`
config key), and ~20 guard tests protecting both flavors.

The two config schemas are mutually exclusive by OpenCode's own rules (v1
rejects the v2 `permissions` array and vice versa), so every config feature had
to be written, reasoned about, and tested twice.

An inventory audit (2026-09-11) found:

- No telemetry, logs, or persisted records of v1 usage anywhere in the repo.
  `evals.ProbeMode` results are never persisted.
- Indirect signals all point to v2: the default binary is `opencode2`,
  the flagship background-agents plugin already refuses v1, and the docs
  recommend `opencode_version v2`.
- The dual-support tax was already producing contradictions: the Settings UI
  claimed background-agents/advisor were "wired only on v1" while the code
  refuses v1.

## Decision

OpenCode v2 (`opencode2`) is the minimum supported version.

1. Gate first: installing/applying against a v1 binary fails with a clear
   error instead of silently writing v2 shapes.
2. Delete the v1 writers (permissions translation, legacy MCP/plugin/provider
   shapes, v1 evals dialect, flavor selector, `YWAI_OPENCODE` /
   `opencode_version`).
3. KEEP the migration readers (dual-key reads, legacy key sweeps, group
   frontmatter migration). They are not v1 support — they migrate v1-era user
   data to v2 shapes on first contact. Deleting them orphans existing user
   configs.
4. Delete or rewrite the v1 guard tests; add migration tests instead.
5. Document the withdrawal here and in the user docs.

## Consequences

- ≈1,050-1,250 lines removed (≈600 production, ≈500 tests, plus UI/docs).
- Anyone running the v1 `opencode` binary gets a clear gate error at
  install/apply time instead of silent breakage.
- New config features no longer need a dual-schema implementation and dual
  tests.
- `ywai clean` remains the one-shot path for old-install artifacts; the
  migration readers keep converting v1-era config data on contact.
- The TS plugin adapters (`createV1ShapedClient`, `mapV2EventToV1`) are NOT
  affected: despite the name, they adapt plugin code to run on v2.
- TokenBank's `/v1/models` API is unrelated and unaffected.
