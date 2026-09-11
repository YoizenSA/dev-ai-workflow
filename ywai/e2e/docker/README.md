# Docker lifecycle matrix for ywai

Real `install` / `update` / `uninstall` lifecycle tests in Docker containers.
Each cell runs in its own container with `HOME=/home/ywai` (UID 10001), fake
agents on PATH, and a contract to prove. No cell ever publishes a port.

## Layout

| Path | What it is |
|---|---|
| `dryrun.Dockerfile` | Tier A image: embedded binary + fakes on `ubuntu:24.04`. Networkless and toolless at run time. |
| `install.Dockerfile` | Tier B image: `golang:1.26` builder into `ubuntu:24.04`, `MODE=plain\|embedded` arg. |
| `fakes/opencode2.sh`, `fakes/claude.sh` | Agent stubs named like the real binaries; they log argv and answer `2.0.0` on `--version`. |
| `lib/asserts.sh` | Assertion helpers and the non-interactive `run_ywai` wrapper (stdin from `/dev/null`). |
| `lib/snapshot.sh` | `find`-based HOME snapshots (path list + sha256 hashes) and diffs. |
| `scenarios/*.sh` | One script per scenario; each runs inside a container as the cell. |
| `compose.yaml` | Every cell as a compose service with `dry`, `net`, `nightly` profiles. |
| `run-matrix.sh` | Builds images, fans out cells, prints a `cell\|result\|duration` table. |

## Cells

Offline release gate — profile `dry` (embedded binary, `network_mode: none`):

| Cell | Agent shape | Scenario |
|---|---|---|
| `dry-opencode2` | opencode2 | `install --dry-run` is clean and writes nothing |
| `dry-claude` | claude | same |
| `dry-both` | both | same |

Online — profile `net` (tier B image, `MODE=plain`):

| Cell | Agent shape | Scenario |
|---|---|---|
| `fresh-opencode2` | opencode2 | fresh install footprint |
| `fresh-claude` | claude | fresh install footprint |
| `fresh-both` | both | fresh install footprint |
| `reinstall-opencode2` | opencode2 | second install leaves HOME unchanged |
| `reinstall-claude` | claude | second install leaves HOME unchanged |
| `update-swap-opencode2` | opencode2 | offline binary-swap update re-applies |
| `update-swap-claude` | claude | offline binary-swap update re-applies |
| `update-swap-both` | both | offline binary-swap update re-applies |

The update-swap cells run with `network_mode: none`: they are the offline,
deterministic update variant, so the real-release self-update must not be
able to reach the network.
| `uninstall-clean-both` | both | `uninstall --yes` removes the managed footprint |

Nightly only — profile `nightly` (the 9 net cells plus):

| Cell | Agent shape | Scenario |
|---|---|---|
| `update-release-opencode2` | opencode2 | real release N-1 -> N via `ywai update` |

`update-swap` and `update-release` are the only two update variants on
purpose: one is deterministic and offline, the other is the real release
path. Nothing in between is honest.

## Run it

```bash
# From ywai/ (or use: bash scripts/dev.sh docker-matrix [profile])
bash e2e/docker/run-matrix.sh --profile dry       # offline, fast, default
bash e2e/docker/run-matrix.sh --profile net       # real installs
bash e2e/docker/run-matrix.sh --profile nightly   # net + real release update

# One cell only
bash e2e/docker/run-matrix.sh --profile net --cell fresh-opencode2

# List the cells a profile would run
bash e2e/docker/run-matrix.sh --profile nightly --list
```

`dev.sh docker-matrix` skips with a clear message when Docker is missing.

## The cell contract

Every real install runs non-interactively: `--agent <name>
--autostart=false --ponytail=false`, stdin from `/dev/null`, no TTY. A bare
`install` opens a TUI, so cells never do that; "both" cells run one install
per target. Uninstall runs `--yes`.

Assertions are filesystem-first:

- `test -f` / directory checks for the installed footprint
- `jq` on `opencode.json` (agents keys, `~/.ywai/version.json` `.installed`)
- HOME snapshots from `lib/snapshot.sh`: file list + sha256 hashes, diffed
  before/after. Only files are tracked: agent detection may leave empty
  `skills/` dirs behind (pre-existing behavior in `internal/agent`), and
  `~/.ywai/version.json` is excluded from hash stability because ywai
  touches it on every command.
- the stable `=== Done! ===` output marker (and `=== Failed` must not appear)

## Fault injection example

Cells are just `bash` in a container, so you inject a fault by overriding the
entrypoint. Prove the harness catches a broken install by destroying state
mid-scenario:

```bash
# Simulate "install deleted the skills dir between installs": the
# reinstall cell must fail its unchanged-HOME assertion.
docker compose -f ywai/e2e/docker/compose.yaml run --rm \
  --entrypoint 'bash -c "rm -rf /home/ywai/.config/opencode && bash /opt/ywai-e2e/scenarios/reinstall.sh"' \
  reinstall-opencode2
# Expect: RESULT [reinstall-opencode2-reinstall]: FAIL and a non-zero exit.
```

Other useful injections:

```bash
# Drop network mid-flight for an online cell: update-swap must still pass
# (its self-update failure is expected to stay soft).
docker compose -f ywai/e2e/docker/compose.yaml run --rm --no-deps \
  --entrypoint 'bash -c "bash /opt/ywai-e2e/scenarios/update-swap.sh"' \
  update-swap-opencode2

# Interactive shell in a built cell image to poke at state by hand:
docker compose -f ywai/e2e/docker/compose.yaml run --rm \
  --entrypoint bash fresh-opencode2
```

To see the diff a failing assertion found, read the `FAIL` block in the cell
output; `assert_home_unchanged` prints a unified diff of the snapshots.

## CI

- `.github/workflows/docker-matrix.yml`: nightly cron plus manual dispatch.
  One job per cell, `fail-fast: false`, 45-minute cap per job.
- `.github/workflows/release.yml`: the `docker-e2e` gate job runs the 3 dry
  cells (embedded binary, networkless) before GoReleaser publishes.

Design rationale: `docs/adr/0002-docker-matrix-lifecycle-tests.md`.
