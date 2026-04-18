#!/usr/bin/env bash
set -e

TARGET_DIR="${1:-.}"
EXT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$EXT_DIR/../../../skills/_shared"
TARGET_SHARED_DIR="$TARGET_DIR/skills/_shared"

if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "Shared skills source not found: $SOURCE_DIR (skipping)"
  exit 0
fi

mkdir -p "$TARGET_SHARED_DIR"

copied=0
for item in "$SOURCE_DIR"/*; do
  [[ -e "$item" ]] || continue
  item_name="$(basename "$item")"
  if [[ ! -e "$TARGET_SHARED_DIR/$item_name" ]]; then
    if [[ -d "$item" ]]; then
      cp -r "$item" "$TARGET_SHARED_DIR/$item_name"
    else
      cp "$item" "$TARGET_SHARED_DIR/$item_name"
    fi
    copied=$((copied + 1))
  fi
done

echo "Installed $copied shared skill asset(s) into skills/_shared/"
