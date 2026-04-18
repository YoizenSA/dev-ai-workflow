#!/usr/bin/env bash
set -e

TARGET_DIR="${1:-.}"
EXT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$EXT_DIR"
TARGET_PROMPTS_DIR="$TARGET_DIR/.github/prompts"
LEGACY_PROMPTS_DIR="$TARGET_DIR/prompts"
TARGET_OPENCODE_SKILLS_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/opencode/skills"
TARGET_OPENCODE_COMMANDS_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/opencode/commands"
TARGET_COPILOT_AGENTS_DIR="$HOME/.copilot/agents"

if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "Commands source not found: $SOURCE_DIR"
  exit 1
fi

mkdir -p "$TARGET_PROMPTS_DIR" "$TARGET_OPENCODE_SKILLS_DIR" "$TARGET_OPENCODE_COMMANDS_DIR" "$TARGET_COPILOT_AGENTS_DIR"

# Migrate legacy prompt location (project-root prompts/) to .github/prompts
if [[ -d "$LEGACY_PROMPTS_DIR" ]]; then
  for file in "$LEGACY_PROMPTS_DIR"/sdd-*.md; do
    [[ -f "$file" ]] || continue
    name="$(basename "$file")"
    [[ -f "$TARGET_PROMPTS_DIR/$name" ]] || mv "$file" "$TARGET_PROMPTS_DIR/$name"
  done
fi

copied=0
for file in "$SOURCE_DIR"/*.md; do
  [[ -f "$file" ]] || continue
  name="$(basename "$file" .md)"

  # Copy to GitHub Copilot prompts (project-local)
  if [[ ! -f "$TARGET_PROMPTS_DIR/$name.md" ]]; then
    cp "$file" "$TARGET_PROMPTS_DIR/$name.md"
    copied=$((copied + 1))
  fi

  # Copy to OpenCode skills directory structure (global)
  skill_dir="$TARGET_OPENCODE_SKILLS_DIR/$name"
  mkdir -p "$skill_dir"
  if [[ ! -f "$skill_dir/SKILL.md" ]]; then
    cp "$file" "$skill_dir/SKILL.md"
    copied=$((copied + 1))
  fi

  # Copy to OpenCode commands (global) — enables slash invocation like /sdd-init
  if [[ ! -f "$TARGET_OPENCODE_COMMANDS_DIR/$name.md" ]]; then
    cp "$file" "$TARGET_OPENCODE_COMMANDS_DIR/$name.md"
    copied=$((copied + 1))
  fi

  # Copy to Copilot agents (global)
  if [[ ! -f "$TARGET_COPILOT_AGENTS_DIR/$name.md" ]]; then
    cp "$file" "$TARGET_COPILOT_AGENTS_DIR/$name.md"
    copied=$((copied + 1))
  fi
done

echo "Installed slash commands to .github/prompts ($TARGET_PROMPTS_DIR), OpenCode skills ($TARGET_OPENCODE_SKILLS_DIR), OpenCode commands ($TARGET_OPENCODE_COMMANDS_DIR), and Copilot agents ($TARGET_COPILOT_AGENTS_DIR)"
