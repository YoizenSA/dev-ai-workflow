#!/usr/bin/env bash
# =============================================================================
# Cell: offline update via deterministic binary swap.
#
# Two binaries are baked into the image, built from the same source and
# differing only in their version string (YWAI_VERSION_OLD < YWAI_VERSION_NEW).
# This is the honest offline variant of an update: no release download is
# faked, none is needed. The nightly update-release cell covers the real
# release N-1 -> N path.
#
# Steps: swap the old build onto PATH, install, swap the new build onto PATH,
# run `ywai update`. Offline self-update prints a warning and falls back to
# `go install`, which also fails without a toolchain: both are expected and
# must stay soft (the command still re-applies everything and exits 0).
# =============================================================================
set -uo pipefail
source "$(cd "$(dirname "$0")" && pwd)/../lib/asserts.sh"
source "$(cd "$(dirname "$0")" && pwd)/../lib/snapshot.sh"

cell_init update-swap

old="${YWAI_BIN_OLD:-/opt/ywai-e2e/bin/ywai-old}"
new="${YWAI_BIN_NEW:-/opt/ywai-e2e/bin/ywai-new}"
old_version="${YWAI_VERSION_OLD:?YWAI_VERSION_OLD must be set by the image}"
new_version="${YWAI_VERSION_NEW:?YWAI_VERSION_NEW must be set by the image}"

assert_file "$old"
assert_file "$new"

echo "-- install with the old build"
# Both builds live in the cell user's own bin dir (on PATH); swap_binary
# replaces the PATH binary in place, even with a process mapped from it.
swap_binary "$old" "$YWAI_BIN"
for t in $TARGETS; do
    run_ywai install --agent "$t" "${INSTALL_FLAGS[@]}"
    rc=$?
    assert_exit_zero "$rc" "old install --agent $t"
    assert_contains "$RUN_OUT" "=== Done! ===" "old install --agent $t prints the stable marker"
done
assert_targets_installed

# Settle before reading the version file: installing for OpenCode also
# starts the control server, and that server's startup Refresh races the
# install's version Touch (a run observed `installed` empty). Every ywai
# command re-Touches version.json with its own version, so one cheap
# command makes the state deterministic again.
run_ywai version >/dev/null 2>&1
assert_json_field "$(version_file)" .installed "$old_version"

echo "-- swap in the new build and update"
swap_binary "$new" "$YWAI_BIN"
for t in $TARGETS; do
    run_ywai update --agent "$t"
    rc=$?
    assert_exit_zero "$rc" "update --agent $t"
    assert_contains "$RUN_OUT" "=== Done! ===" "update --agent $t prints the stable marker"
    assert_not_contains "$RUN_OUT" "=== Failed" "update --agent $t has no failed footer"
done

# Managed state survives the update and the version file records the new build.
stop_serve
assert_targets_installed
assert_json_field "$(version_file)" .installed "$new_version"
finish
