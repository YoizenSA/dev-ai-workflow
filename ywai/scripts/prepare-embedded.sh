#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WEB_DIR="$REPO_ROOT/internal/control/web"
EMBED_DIR="$REPO_ROOT/cmd/ywai/embedded_data"

# Rebuild the React UI so the embedded binary always carries the current
# frontend. Without this, `ywai install` / `dev.sh install` would ship a stale
# dist/ (only reload-control.sh rebuilt it before). Skipped in CI when node is
# unavailable — the prebuilt dist/ is still copied as a fallback.
if command -v npm >/dev/null 2>&1; then
    echo "Building React UI (hashed assets)…"
    npm --prefix "$WEB_DIR" run build
else
    echo "npm not found — using existing dist/ as-is"
fi

rm -rf "$EMBED_DIR"
mkdir -p "$EMBED_DIR/skills"
mkdir -p "$EMBED_DIR/agents"
mkdir -p "$EMBED_DIR/ui"

cp -a "$REPO_ROOT/skills/." "$EMBED_DIR/skills/"
cp -a "$REPO_ROOT/agents/." "$EMBED_DIR/agents/"
cp -a "$WEB_DIR/dist/." "$EMBED_DIR/ui/"

skill_count=$(ls -d "$EMBED_DIR/skills"/*/ 2>/dev/null | wc -l)
agent_count=$(find "$EMBED_DIR/agents" -name "AGENT.md" | wc -l)
echo "Prepared embedded data: $skill_count skills, $agent_count agent profiles, control UI (React)"
