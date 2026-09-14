#!/usr/bin/env bash
# =============================================================================
# Cell: reinstall (idempotency).
#
# Contract: installing twice on the same HOME ends with exit 0, the stable
# marker, and an unchanged HOME: the second run must rewrite managed files
# with identical content and create or delete nothing else.
# =============================================================================
set -uo pipefail
source "$(cd "$(dirname "$0")" && pwd)/../lib/asserts.sh"
source "$(cd "$(dirname "$0")" && pwd)/../lib/snapshot.sh"

cell_init reinstall

echo "-- first install"
for t in $TARGETS; do
    install_target "$t"
done
assert_targets_installed

# install leaves a detached control server behind; a live serve writes
# state files and would show up as snapshot churn.
stop_serve

before="/tmp/${CELL_NAME}-1.txt"
snapshot_home "$before"

echo "-- second install (idempotency)"
for t in $TARGETS; do
    install_target "$t"
done

stop_serve
assert_targets_installed
assert_home_unchanged "second install" "$before"
finish
