#!/usr/bin/env bash
# =============================================================================
# Docker lifecycle matrix runner for ywai.
#
# Builds the cell images, runs every cell of the selected profile, and prints
# a cell|result|duration table. Exits non-zero and lists the failing cells
# when any cell fails.
#
# Usage:
#   run-matrix.sh [--profile dry|net|nightly] [--cell NAME]... [--list]
#
# Profiles:
#   dry     offline dry-run cells (default; the release-gate profile)
#   net     online install / reinstall / update-swap / uninstall cells
#   nightly net cells plus the real release N-1 -> N update cell
#
# Requirements: Docker with compose v2. Every cell runs in its own container
# with HOME=/home/ywai (UID 10001); nothing is published on any port.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# YWAI_COMPOSE_FILE lets a caller pass a daemon-visible path (needed when the
# docker CLI is a Windows binary driven from WSL bash).
COMPOSE_FILE="${YWAI_COMPOSE_FILE:-$SCRIPT_DIR/compose.yaml}"
COMPOSE=(docker compose -f "$COMPOSE_FILE")

DRY_CELLS=(dry-opencode2 dry-claude dry-both)
NET_CELLS=(
    fresh-opencode2 fresh-claude fresh-both
    reinstall-opencode2 reinstall-claude
    update-swap-opencode2 update-swap-claude update-swap-both
    uninstall-clean-both
)
NIGHTLY_CELLS=("${NET_CELLS[@]}" update-release-opencode2)

PROFILE=dry
CELL_FILTERS=()
LIST_ONLY=0

usage() {
    sed -n '2,19p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [ $# -gt 0 ]; do
    case "$1" in
    --profile)
        PROFILE="${2:?--profile needs a value}"
        shift 2
        ;;
    --cell)
        CELL_FILTERS+=("${2:?--cell needs a value}")
        shift 2
        ;;
    --list)
        LIST_ONLY=1
        shift
        ;;
    -h | --help)
        usage
        exit 0
        ;;
    *)
        echo "unknown option: $1" >&2
        usage
        exit 2
        ;;
    esac
done

case "$PROFILE" in
dry) CELLS=("${DRY_CELLS[@]}") ;;
net) CELLS=("${NET_CELLS[@]}") ;;
nightly) CELLS=("${NIGHTLY_CELLS[@]}") ;;
*)
    echo "unknown profile: $PROFILE (want dry, net or nightly)" >&2
    exit 2
    ;;
esac

# --cell selects a subset. A named cell outside the profile is skipped with a
# clear line, not an error: the CI matrix passes the same --profile for every
# cell and lets this filter decide what actually runs.
SELECTED=()
if [ "${#CELL_FILTERS[@]}" -eq 0 ]; then
    SELECTED=("${CELLS[@]}")
else
    for want in "${CELL_FILTERS[@]}"; do
        found=0
        for cell in "${CELLS[@]}"; do
            if [ "$cell" = "$want" ]; then
                SELECTED+=("$cell")
                found=1
                break
            fi
        done
        if [ "$found" -eq 0 ]; then
            echo "skip: cell $want is not in profile $PROFILE"
        fi
    done
    if [ "${#SELECTED[@]}" -eq 0 ]; then
        echo "nothing to run: no requested cell is in profile $PROFILE"
        exit 0
    fi
fi

if [ "$LIST_ONLY" -eq 1 ]; then
    printf '%s\n' "${SELECTED[@]}"
    exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "FAIL: docker not found. The lifecycle matrix needs Docker." >&2
    exit 1
fi
if ! docker info >/dev/null 2>&1; then
    echo "FAIL: docker daemon not reachable. Start Docker and retry." >&2
    exit 1
fi

echo "== [build] images for profile $PROFILE =="
"${COMPOSE[@]}" --profile "$PROFILE" build "${SELECTED[@]}"

RESULTS=()
FAILED=()
for cell in "${SELECTED[@]}"; do
    echo ""
    echo "== [cell] $cell =="
    start="$(date +%s)"
    if "${COMPOSE[@]}" run --rm -T "$cell"; then
        result=PASS
    else
        result=FAIL
        FAILED+=("$cell")
    fi
    end="$(date +%s)"
    RESULTS+=("${cell}|${result}|$((end - start))s")
done

echo ""
echo "== results (cell|result|duration) =="
for line in "${RESULTS[@]}"; do
    echo "$line"
done

if [ "${#FAILED[@]}" -gt 0 ]; then
    echo ""
    echo "FAILING CELLS: ${FAILED[*]}"
    exit 1
fi

echo ""
echo "All ${#SELECTED[@]} cell(s) passed (profile: $PROFILE)."
