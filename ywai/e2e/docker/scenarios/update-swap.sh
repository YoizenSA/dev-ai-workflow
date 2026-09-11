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
# /usr/local/bin/ywai is chown ywai in the image so a cell can swap it.
cp "$old" /usr/local/bin/ywai
for t in $TARGETS; do
    run_ywai install --agent "$t" "${INSTALL_FLAGS[@]}"
    rc=$?
    assert_exit_zero "$rc" "old install --agent $t"
    assert_contains "$RUN_OUT" "=== Done! ===" "old install --agent $t prints the stable marker"
done
assert_targets_installed
assert_json_field "$(version_file)" .installed "$old_version"

echo "-- swap in the new build and update"
cp "$new" /usr/local/bin/ywai
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
