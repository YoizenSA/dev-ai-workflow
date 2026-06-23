#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WEB_DIR="$REPO_ROOT/internal/control/web"
EMBED_DIR="$REPO_ROOT/cmd/ywai/embedded_data"
BA_DIR="$REPO_ROOT/plugins/background-agents"
BA_BUNDLE="$BA_DIR/dist/background-agents.js"

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

# Build the opencode background-agents plugin into a single self-contained
# bundle. opencode loads it from disk, so it must inline every dependency
# (peers included) — drop --external flags or it will fail to resolve at load.
# Mirrors the npm fallback above: if bun is missing but a prior bundle exists,
# reuse it; otherwise warn and ship without the plugin rather than fail.
# Buscar bun en paths no estándar (CI, shells no interactivos, etc.)
if ! command -v bun >/dev/null 2>&1; then
    for candidate in "$HOME/.bun/bin/bun" "/opt/homebrew/bin/bun" "/usr/local/bin/bun"; do
        if [ -x "$candidate" ]; then
            export PATH="$(dirname "$candidate"):$PATH"
            break
        fi
    done
fi
if command -v bun >/dev/null 2>&1; then
    echo "Building background-agents plugin (bun bundle)…"
    bun install --cwd "$BA_DIR"
    bun build "$BA_DIR/src/plugin/background-agents.ts" \
        --outfile "$BA_BUNDLE" --target node
elif [ -f "$BA_BUNDLE" ]; then
    echo "bun not found — using existing background-agents bundle as-is"
else
    echo "ERROR: bun not found and no prebuilt background-agents bundle." >&2
    echo "       The background-agents plugin is required; refusing to ship a release without it." >&2
    echo "       Install bun (https://bun.sh) or commit plugins/background-agents/dist/background-agents.js." >&2
    exit 1
fi

rm -rf "$EMBED_DIR"
mkdir -p "$EMBED_DIR/skills"
mkdir -p "$EMBED_DIR/agents"
mkdir -p "$EMBED_DIR/ui"
mkdir -p "$EMBED_DIR/plugins"

cp -a "$REPO_ROOT/skills/." "$EMBED_DIR/skills/"
cp -a "$REPO_ROOT/agents/." "$EMBED_DIR/agents/"
cp -a "$WEB_DIR/dist/." "$EMBED_DIR/ui/"
if [ -f "$BA_BUNDLE" ]; then
    cp -a "$BA_BUNDLE" "$EMBED_DIR/plugins/background-agents.js"
fi

skill_count=$(ls -d "$EMBED_DIR/skills"/*/ 2>/dev/null | wc -l)
agent_count=$(find "$EMBED_DIR/agents" -name "AGENT.md" | wc -l)
plugin_count=$(ls "$EMBED_DIR/plugins"/*.js 2>/dev/null | wc -l || echo 0)
echo "Prepared embedded data: $skill_count skills, $agent_count agent profiles, $plugin_count plugins, control UI (React)"
