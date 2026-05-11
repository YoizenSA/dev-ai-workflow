#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EMBED_DIR="$REPO_ROOT/cmd/ywai/embedded_data"

rm -rf "$EMBED_DIR"
mkdir -p "$EMBED_DIR/skills"

cp -a "$REPO_ROOT/skills/." "$EMBED_DIR/skills/"

skill_count=$(ls -d "$EMBED_DIR/skills"/*/ 2>/dev/null | wc -l)
echo "Prepared embedded data: $skill_count skills"
