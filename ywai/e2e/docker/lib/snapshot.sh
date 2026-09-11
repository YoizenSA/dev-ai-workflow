#!/usr/bin/env bash
# =============================================================================
# HOME filesystem snapshots for the lifecycle cells.
#
# A snapshot file has two sorted sections: a file path list and sha256
# hashes. Assertions are filesystem-first: a cell takes a snapshot, does
# lifecycle work, takes another, and diffs them.
#
# Only FILES are tracked. Empty directories are deliberately invisible:
# agent detection materializes empty skills/ dirs as a side effect
# (internal/agent createSkillsDir), which is pre-existing behavior no
# lifecycle cell needs to police. Content shows up as files anyway.
#
# ~/.ywai/version.json is volatile by design (ywai touches it on every
# command), so it is excluded from the hash section. Cells that care about
# it assert its .installed field explicitly with jq.
#
# SNAPSHOT_PRUNE_DIRS (optional, space-separated absolute paths) prunes whole
# subtrees from both sections. The dry-run cell prunes $HOME/.ywai: seeding
# that cache happens in PersistentPreRun before the dry run (root.go), so it
# is not a side effect of the dry run itself.
#
# Needs: find with -printf (GNU findutils, present in ubuntu:24.04) and
# sha256sum (coreutils).
# =============================================================================

SNAPSHOT_PRUNE_DIRS=""

snapshot_home() {
    local out="$1"
    local prune_args=() d
    for d in $SNAPSHOT_PRUNE_DIRS; do
        prune_args+=(-path "$d" -prune -o)
    done
    {
        echo "# begin files"
        find "$HOME" "${prune_args[@]}" -path "$YWAI_FAKE_LOG" -prune -o \
            -type f -printf '%P\n' | LC_ALL=C sort
        echo "# begin hash"
        find "$HOME" "${prune_args[@]}" -path "$YWAI_FAKE_LOG" -prune -o -type f -print \
            | grep -v -F "$HOME/.ywai/version.json" \
            | while IFS= read -r f; do sha256sum "$f"; done | LC_ALL=C sort -k 2
    } >"$out"
}

# assert_home_unchanged BEFORE_LABEL BEFORE_FILE
# Takes a fresh snapshot and diffs it against BEFORE_FILE.
assert_home_unchanged() {
    local label="$1" before="$2" after="/tmp/${CELL_NAME}-after.txt" d
    snapshot_home "$after"
    if d="$(diff -u "$before" "$after")"; then
        pass "home unchanged after $label"
    else
        fail "home changed during $label:"
        printf '%s\n' "$d" | head -n 40 | sed 's/^/       /'
    fi
}
