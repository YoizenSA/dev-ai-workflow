#!/usr/bin/env bash
# =============================================================================
# ywai — Full control-dashboard reload
# =============================================================================
#
# One command that rebuilds the React UI, re-embeds it, rebuilds the Go binary
# with the embedded UI, and restarts the control server. Assets are content-
# hashed, so the browser picks up the new build on a normal reload (no hard
# refresh needed).
#
# Usage:
#   ./scripts/reload-control.sh
#
# Env overrides:
#   PORT   server port            (default: 5768)
#   BIN    output binary path     (default: /tmp/ywai-test)
#   ARGS   extra serve args       (default: --no-mcp)
#   LOG    server log file        (default: /tmp/ywai-test.log)
# =============================================================================

set -euo pipefail

PORT="${PORT:-5768}"
BIN="${BIN:-/tmp/ywai-test}"
ARGS="${ARGS:---no-mcp}"
LOG="${LOG:-/tmp/ywai-test.log}"

# ---------------------------------------------------------------------------
# Locate the ywai module root (the dir whose go.mod declares the ywai module).
# ---------------------------------------------------------------------------
find_root() {
    local dir
    dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
    while [[ "$dir" != "/" ]]; do
        if [[ -f "$dir/go.mod" ]] && grep -q 'module github.com/Yoizen/dev-ai-workflow/ywai' "$dir/go.mod" 2>/dev/null; then
            echo "$dir"; return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo "error: ywai module root not found" >&2; exit 1
}

ROOT="$(find_root)"
cd "$ROOT"

echo "==> [1/5] Building React UI (hashed assets)…"
npm --prefix internal/control/web run build

echo "==> [2/5] Preparing embedded data…"
bash scripts/prepare-embedded.sh

echo "==> [3/5] Building embedded binary → ${BIN}…"
go build -tags embedded -o "${BIN}" ./cmd/ywai

echo "==> [4/5] Stopping any server on :${PORT}…"
# Kill the previous instance of THIS script's binary first (fast path).
pkill -f "$(basename "${BIN}") serve --port ${PORT}" 2>/dev/null || true
# Then kill ANY process still listening on the port (covers an independently
# installed ywai, a stale serve, etc.). macOS has no `ss`; use lsof.
if command -v lsof >/dev/null 2>&1; then
    while read -r pid; do
        [[ -n "$pid" ]] && kill "$pid" 2>/dev/null || true
    done < <(lsof -nP -iTCP:"${PORT}" -sTCP:LISTEN -t 2>/dev/null)
fi
# Wait for the port to free up (up to ~5s).
port_busy() {
    if command -v ss >/dev/null 2>&1; then
        ss -tln 2>/dev/null | grep -q ":${PORT} "
    elif command -v lsof >/dev/null 2>&1; then
        lsof -nP -iTCP:"${PORT}" -sTCP:LISTEN -t 2>/dev/null | grep -q .
    else
        return 1
    fi
}
for _ in $(seq 1 10); do
    if ! port_busy; then break; fi
    sleep 0.5
done

echo "==> [5/5] Starting server: ${BIN} serve --port ${PORT} ${ARGS}"
# shellcheck disable=SC2086
nohup "${BIN}" serve --port "${PORT}" ${ARGS} > "${LOG}" 2>&1 &
sleep 2

if port_busy; then
    echo "✓ Control server listening on http://localhost:${PORT}  (log: ${LOG})"
    echo "  Reload the browser normally — hashed assets bust the cache."
else
    echo "✗ Server failed to start. Last log lines:" >&2
    tail -n 20 "${LOG}" >&2 || true
    exit 1
fi
