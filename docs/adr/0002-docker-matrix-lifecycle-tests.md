# ADR-0002: Docker-based lifecycle matrix tests for install/update/uninstall

## Status

Proposed (2026-09-11)

## Context

ywai's `install` and `update` share one pipeline (`applyManaged`,
`cmd/ywai/apply.go`) that writes into ~15 user-facing locations: agent
configs, skills and agents directories, plugin bundles, autostart, AGENTS.md
and `~/.ywai`. A regression there corrupts real user setups.

The Go e2e suite (`ywai/e2e/`) covers only `--dry-run` and help-text output.
It asserts on stdout, never on filesystem state; it builds without the
`embedded` tag and runs from the repo checkout, so both the shipped binary
mode and "fresh machine" behavior are untested. Update and uninstall had zero
end-to-end coverage.

Container behavior also differs from dev machines in ways unit tests cannot
see: no systemd (autostart warnings flip the exit code to 1, `apply.go`
warning handling), no TTY (the install wizard gate and the uninstall
confirmation), and the OpenCode binary must resolve as `opencode2`
(ADR-0001).

## Decision

1. Lifecycle tests run in Docker, one container per cell, under
   `ywai/e2e/docker/`, harnessed by bash + compose (`run-matrix.sh`).
2. Two images: a toolless, networkless dry-run tier (proves the dry run has
   no hidden network or toolchain dependency) and a multi-stage install tier
   that builds ywai in-image with a `MODE=plain|embedded` build arg.
3. Fake agents are `sh` stubs named `opencode2` and `claude` on PATH
   (per ADR-0001); every real install runs `--agent <name> --autostart=false
   --ponytail=false` with stdin from `/dev/null`; uninstall runs `--yes`.
4. Assertions are filesystem-first: installed footprint, `jq` on
   `opencode.json` and `~/.ywai/version.json`, and before/after HOME
   snapshots (`lib/snapshot.sh`). Output assertions use only stable markers
   (`=== Done! ===`), never the dynamic step numbering.
5. Update coverage has exactly two honest variants: an offline binary-swap
   (two builds from the same source differing only in version string, run in
   `network_mode: none` containers so self-update cannot fetch a real
   release) and a nightly real release N-1 -> N cell. A local dev binary plus
   network is never tested — self-update would silently fetch the public
   release.
6. CI: nightly full matrix (13 cells, `fail-fast: false`) plus manual
   dispatch. The Docker matrix is optional and does not gate tag publishes;
   `release.yml` waits only on lint + Go tests before GoReleaser.
7. Non-goals: Windows containers (GoReleaser covers the build; the install
   path is bash), real agent CLIs (they need API keys), systemd-enabled
   images, and per-PR runs.

## Consequences

- Installer, updater and uninstaller regressions surface as red cells on
  real Linux with isolated HOMEs instead of reaching releases undetected.
- A new bash harness lives next to the Go tests and needs maintenance.
- Nightly cells that touch GitHub releases can flake on API rate limits;
  the release-path cell retries and fails with a clear message.
- A red Docker cell no longer blocks a tag publish; catch regressions on
  the nightly/manual workflow instead.
- Follow-ups: unify warning semantics (some failures print "Warning:" and
  exit 0 while recorded warnings exit 1), and consider an env escape hatch
  to skip self-update in tests.
