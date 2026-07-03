#!/usr/bin/env bash
# dev-rebuild.sh — rebuild web + binary, (re)install, and restart the control
# server so local changes are testable end to end. Idempotent: stops any running
# server first, starts a fresh one, and verifies it responds.
#
# Usage:
#   ywai/scripts/dev-rebuild.sh            # full: web + binary + restart
#   ywai/scripts/dev-rebuild.sh --no-web   # skip the (slow) web build
#   ywai/scripts/dev-rebuild.sh --no-serve # build+install only, don't restart
set -euo pipefail

# Resolve the ywai module root from this script's location, independent of cwd.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
YWAI_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
WEB_DIR="$YWAI_DIR/internal/control/web"
UI_EMBED="$YWAI_DIR/cmd/ywai/embedded_data/ui"
PORT="${YWAI_PORT:-5768}"

BUILD_WEB=1
DO_SERVE=1
for arg in "$@"; do
	case "$arg" in
		--no-web) BUILD_WEB=0 ;;
		--no-serve) DO_SERVE=0 ;;
		*) echo "unknown flag: $arg" >&2; exit 2 ;;
	esac
done

cd "$YWAI_DIR"

if [[ "$BUILD_WEB" == 1 ]]; then
	echo "▶ Building web frontend…"
	# --config.verify-deps-before-run=false: pnpm otherwise runs an implicit
	# dependency check (and install) before the script, which fails in a
	# restricted/offline environment even when node_modules is already valid.
	(cd "$WEB_DIR" && pnpm --config.verify-deps-before-run=false build)
	echo "▶ Syncing dist → embedded_data/ui…"
	rm -rf "$UI_EMBED"
	mkdir -p "$UI_EMBED"
	cp -r "$WEB_DIR/dist/." "$UI_EMBED/"
else
	echo "▶ Skipping web build (--no-web)"
fi

# Install to the SAME path `serve` runs from. `ywai serve` self-updates and
# re-execs its canonical install location (~/.local/bin/ywai), so a plain
# `go install` into ~/go/bin would be ignored. Target that path directly and
# start with --no-update so our freshly built binary is the one that runs.
INSTALL_DIR="$(dirname "$(command -v ywai 2>/dev/null || echo "$HOME/.local/bin/ywai")")"
mkdir -p "$INSTALL_DIR"
echo "▶ Installing ywai (go install -tags embedded) → $INSTALL_DIR…"
GOBIN="$INSTALL_DIR" go install -tags embedded ./cmd/ywai
YWAI_BIN="$INSTALL_DIR/ywai"

if [[ "$DO_SERVE" == 0 ]]; then
	echo "✓ Built & installed to $YWAI_BIN. Skipped restart (--no-serve)."
	exit 0
fi

echo "▶ Restarting control server ($YWAI_BIN serve --no-update --background)…"
"$YWAI_BIN" stop >/dev/null 2>&1 || true
"$YWAI_BIN" serve --no-update --background

# Verify the server is up.
echo -n "▶ Waiting for http://localhost:$PORT/ "
for _ in $(seq 1 15); do
	if [[ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$PORT/" --max-time 2)" == "200" ]]; then
		echo "→ up"
		echo "✓ Ready: http://localhost:$PORT/"
		exit 0
	fi
	echo -n "."
	sleep 1
done
echo
echo "✗ server did not respond on port $PORT" >&2
exit 1
