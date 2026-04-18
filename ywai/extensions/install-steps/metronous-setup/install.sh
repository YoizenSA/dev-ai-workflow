#!/usr/bin/env bash
# Metronous Setup Extension — macOS / Linux
# Installs metronous CLI and configures OpenCode telemetry.
set -e

TARGET_DIR="${1:-.}"
EXT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="$TARGET_DIR/.ywai/metronous"
STATUS_FILE="$STATE_DIR/status.txt"
README_FILE="$STATE_DIR/README.md"

log() { printf "[metronous-setup] %s\n" "$*"; }
warn() { printf "[metronous-setup] WARN: %s\n" "$*" >&2; }

mkdir -p "$STATE_DIR"

cat > "$README_FILE" << 'EOF'
# Metronous Setup

This project uses the `metronous-setup` extension for agent telemetry and benchmarking.

Metronous has been installed and configured for OpenCode.

## What was configured

- Metronous CLI installed
- OpenCode configured with metronous MCP shim
- Metronous plugin installed to ~/.config/opencode/plugins/
- Daemon service configured (systemd on Linux)

## Next steps

Start the metronous dashboard:

```bash
metronous dashboard
```

The dashboard has 5 tabs for tracking, benchmarks, costs, config, and reports.

## References

- Repo: https://github.com/kiosvantra/metronous
- Docs: https://github.com/kiosvantra/metronous
EOF

# ---------------------------------------------------------------------------
# 1. Install metronous CLI
# ---------------------------------------------------------------------------
if command -v metronous >/dev/null 2>&1; then
  version="$(metronous --version 2>/dev/null || echo present)"
  log "metronous CLI already installed: $version"
else
  if command -v curl >/dev/null 2>&1; then
    log "Installing metronous CLI from official install script"
    if curl -fsSL https://github.com/kiosvantra/metronous/releases/latest/download/install.sh | bash; then
      log "metronous CLI installed"
    else
      warn "metronous CLI install failed"
      cat > "$STATUS_FILE" << EOF
metronous: install_failed
auto_configured: no
note: automatic install failed
EOF
      exit 0
    fi
  else
    warn "curl not available — cannot install metronous CLI automatically"
    cat > "$STATUS_FILE" << EOF
metronous: install_failed
auto_configured: no
note: curl not available
EOF
    exit 0
  fi
fi

# ---------------------------------------------------------------------------
# 2. Run metronous install (configures OpenCode automatically)
# ---------------------------------------------------------------------------
if command -v metronous >/dev/null 2>&1; then
  log "Running metronous install to configure OpenCode"
  if metronous install >/dev/null 2>&1; then
    log "OpenCode configured with metronous"
    configured=1
  else
    warn "metronous install failed - you may need to run it manually"
    configured=0
  fi
else
  warn "metronous CLI not available after install attempt"
  configured=0
fi

version="$(metronous --version 2>/dev/null || echo unknown)"

cat > "$STATUS_FILE" << EOF
metronous: installed
version: ${version}
auto_configured: yes
configured: ${configured}
EOF

log "Done (configured: $configured)"
