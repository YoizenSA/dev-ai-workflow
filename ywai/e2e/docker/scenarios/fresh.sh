#!/usr/bin/env bash
# =============================================================================
# Cell: fresh install on a clean HOME.
#
# Contract: every target installs non-interactively with
# `--agent <name> --autostart=false --ponytail=false`, exits 0, prints the
# stable marker, and leaves the expected filesystem footprint.
# =============================================================================
set -uo pipefail
source "$(cd "$(dirname "$0")" && pwd)/../lib/asserts.sh"

cell_init fresh

# Precondition, not assumption: the container HOME starts clean.
for d in "$HOME/.config/opencode" "$HOME/.claude" "$HOME/.ywai"; do
    if [ -e "$d" ]; then
        fail "precondition: $d already exists on a fresh container"
    fi
done
pass "precondition: HOME is clean"

for t in $TARGETS; do
    install_target "$t"
done

assert_targets_installed

# The fake agents log their argv the first time ywai probes them.
assert_file "$YWAI_FAKE_LOG"
finish
