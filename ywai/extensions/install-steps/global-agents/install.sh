#!/usr/bin/env bash
# Global Agents Extension - Linux/macOS
#
# Preferred path: delegate to the `ywai` binary which runs the in-process
# globalagents generator (same code used by the wizard). The fallback copies
# templates directly but preserves user-owned files (any .md not matching a
# template basename stays untouched).

set -e

TARGET_DIR="${1:-.}"
PROJECT_TYPE="${YWAI_PROJECT_TYPE:-generic}"
EXT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SOURCE="$EXT_DIR/templates"
VERSION_FILE="$AGENTS_SOURCE/VERSION"
STATE_DIR="$HOME/.ywai"
STATE_VERSION_FILE="$STATE_DIR/global-agents-version"

echo "Configuring global agents for project type: $PROJECT_TYPE"

# --- Try the Go binary first --------------------------------------------------
if command -v ywai >/dev/null 2>&1; then
    echo "Delegating to: ywai --update-global-agents --type=$PROJECT_TYPE --silent"
    if ywai --update-global-agents --type="$PROJECT_TYPE" --silent; then
        if [[ -f "$VERSION_FILE" ]]; then
            mkdir -p "$STATE_DIR"
            tr -d '[:space:]' < "$VERSION_FILE" > "$STATE_VERSION_FILE"
        fi
        exit 0
    fi
    echo "ywai delegation failed, falling back to shell implementation"
fi

# --- Fallback: copy templates, preserving user-owned files --------------------
if [[ ! -d "$AGENTS_SOURCE" ]]; then
    echo "Agent templates not found: $AGENTS_SOURCE"
    exit 1
fi

LOCAL_VERSION=""
if [[ -f "$VERSION_FILE" ]]; then
    LOCAL_VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
fi

INSTALLED_VERSION=""
if [[ -f "$STATE_VERSION_FILE" ]]; then
    INSTALLED_VERSION="$(tr -d '[:space:]' < "$STATE_VERSION_FILE")"
fi

if [[ -n "$LOCAL_VERSION" && "$LOCAL_VERSION" == "$INSTALLED_VERSION" ]]; then
    echo "Global agents already up to date (version $INSTALLED_VERSION)"
    echo "To force reinstall, remove $STATE_VERSION_FILE"
    exit 0
fi

HOME_DIR="${HOME}"
XDG_CONFIG="${XDG_CONFIG_HOME:-$HOME_DIR/.config}"

declare -A AGENT_LOCATIONS=(
    ["OpenCode"]="$XDG_CONFIG/opencode/agent"
    ["Copilot"]="$HOME_DIR/.copilot/agents"
    ["Claude"]="$HOME_DIR/.claude/agents"
    ["Agents"]="$HOME_DIR/.agents/agents"
)
# Gemini and Cursor are intentionally excluded from the managed agent set:
# the current policy is to only support OpenCode, Claude, and Copilot at the
# global level. Users wanting Gemini/Cursor can still keep their own files
# in those directories — this script leaves them untouched.

# Managed basenames: only these are removed/overwritten. User-owned .md files
# survive re-runs.
MANAGED_FILES=()
for agent_file in "$AGENTS_SOURCE"/*.md; do
    [[ -f "$agent_file" ]] || continue
    MANAGED_FILES+=("$(basename "$agent_file")")
done

copied_total=0
for platform_name in "${!AGENT_LOCATIONS[@]}"; do
    dest_dir="${AGENT_LOCATIONS[$platform_name]}"
    mkdir -p "$dest_dir"

    for managed in "${MANAGED_FILES[@]}"; do
        rm -f "$dest_dir/$managed"
    done

    for agent_file in "$AGENTS_SOURCE"/*.md; do
        [[ -f "$agent_file" ]] || continue
        cp -f "$agent_file" "$dest_dir/"
        echo "  [$platform_name] Installed agent: $(basename "$agent_file")"
        copied_total=$((copied_total + 1))
    done
done

echo ""
echo "Global agents configured ($copied_total templates copied)"
echo ""
echo "Locations:"
for platform_name in "${!AGENT_LOCATIONS[@]}"; do
    echo "  $platform_name: ${AGENT_LOCATIONS[$platform_name]}"
done

if [[ -n "$LOCAL_VERSION" ]]; then
    mkdir -p "$STATE_DIR"
    echo "$LOCAL_VERSION" > "$STATE_VERSION_FILE"
    echo ""
    echo "Installed global agents version: $LOCAL_VERSION"
fi
