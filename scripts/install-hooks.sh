#!/usr/bin/env bash
# Point this clone at committed hooks so lint fails before commit/push.
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
chmod +x \
    "$ROOT/.githooks/pre-commit" \
    "$ROOT/.githooks/pre-push" \
    "$ROOT/ywai/scripts/lint.sh"
git -C "$ROOT" config core.hooksPath .githooks
echo "Git hooks installed (core.hooksPath=.githooks)."
echo "pre-commit: golangci-lint on staged ywai/*.go"
echo "pre-push:   go vet + golangci-lint (same as CI)"
