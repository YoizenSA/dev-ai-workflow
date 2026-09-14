#!/usr/bin/env bash
# =============================================================================
# Cell: install --dry-run.
#
# Contract: a dry run on a clean HOME completes cleanly, prints the stable
# `=== Done! ===` marker, and writes NOTHING outside ywai's own data dir.
# It runs in the tier A image: no network (compose network_mode: none) and
# no tools (no git, go, node or jq on PATH).
#
# ~/.ywai is pruned from the snapshot: PersistentPreRun seeds that cache and
# touches version.json before the dry run itself (root.go), on every
# command. The user-facing promise under test is that agent configs
# (~/.config/opencode, ~/.claude) stay untouched and uncreated.
# =============================================================================
set -uo pipefail
source "$(cd "$(dirname "$0")" && pwd)/../lib/asserts.sh"
source "$(cd "$(dirname "$0")" && pwd)/../lib/snapshot.sh"

cell_init dryrun
SNAPSHOT_PRUNE_DIRS="$HOME/.ywai"

before="/tmp/${CELL_NAME}-before.txt"
snapshot_home "$before"

# Bare `install` opens a TUI, so every run targets one agent explicitly.
for t in $TARGETS; do
    run_ywai install --agent "$t" --dry-run
    rc=$?
    assert_exit_zero "$rc" "install --agent $t --dry-run"
    assert_contains "$RUN_OUT" "=== Done! ===" "dry-run prints the stable marker"
    assert_not_contains "$RUN_OUT" "FATAL" "dry-run has no fatal errors"
    assert_not_contains "$RUN_OUT" "=== Failed" "dry-run has no failed footer"
done

assert_home_unchanged "install --dry-run" "$before"

# The stronger half of the contract: no agent config materialized. Agent
# detection may leave EMPTY skills dirs behind (internal/agent
# createSkillsDir, pre-existing behavior the snapshot ignores), but no
# config file may exist.
cfg="$(opencode_config_file)"
if [ -n "$cfg" ]; then
    fail "dry-run created an agent config: $cfg"
else
    pass "no agent config file created"
fi
finish
