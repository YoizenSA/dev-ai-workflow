#!/usr/bin/env bash
# =============================================================================
# ywai — Local development script
# =============================================================================
#
# Usage:  ./scripts/dev.sh <subcommand>
#
# Subcommands:
#   test         Run all tests (go test ./... -v)
#   test-kanban  Run only kanban tests
#   build        Quick build WITHOUT embedded data (fast iteration)
#   build-full   Full build WITH embedded data (prepare + -tags embedded)
#   install      Full build + install to GOPATH
#   check        Full pipeline: test → build-full → verify → install
#   kanban       Build + install + start daemon with UI on port 5768
#   mcp-test     Build + install + verify MCP daemon responds
#   version      Print the version string that would be used
#   help         Show this usage message
#
# Requirements:
#   - Run from the ywai/ directory (script auto-detects it)
#   - Go 1.22+ installed
#
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Auto-detect ywai root directory
# ---------------------------------------------------------------------------
find_ywai_root() {
    local dir
    dir="$(pwd)"
    while [[ "$dir" != "/" ]]; do
        if [[ -f "$dir/go.mod" ]] && grep -q 'module github.com/Yoizen/dev-ai-workflow/ywai' "$dir/go.mod" 2>/dev/null; then
            echo "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo "" >&2
    return 1
}

YWAI_ROOT="$(find_ywai_root)" || {
    echo "❌ ERROR: Cannot find ywai project root (no go.mod with module github.com/Yoizen/dev-ai-workflow/ywai found)"
    echo "   Run this script from within the ywai/ directory tree."
    exit 1
}

cd "$YWAI_ROOT"

# ---------------------------------------------------------------------------
# Colors
# ---------------------------------------------------------------------------
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

ok()   { echo -e "${GREEN}[✓]${NC} $*"; }
info() { echo -e "${YELLOW}[i]${NC} $*"; }
fail() { echo -e "${RED}[✗]${NC} $*"; }
cmd()  { echo -e "${CYAN}[>]${NC} $*"; }

# ---------------------------------------------------------------------------
# Version string
# ---------------------------------------------------------------------------
compute_version() {
    local hash
    hash="$(git rev-parse --short HEAD 2>/dev/null || true)"
    if [[ -n "$hash" ]]; then
        echo "dev-${hash}"
    else
        echo "dev-unknown"
    fi
}

# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------

do_test() {
    info "Running all tests..."
    cmd "go test ./... -v"
    go test ./... -v
    ok "All tests passed"
}

do_test_kanban() {
    info "Running kanban tests..."
    cmd "go test ./internal/kanban/... -v"
    go test ./internal/kanban/... -v
    ok "Kanban tests passed"
}

do_build() {
    local version
    version="$(compute_version)"
    info "Building ywai (quick, no embedded data)..."
    cmd "go build -ldflags=\"-X main.version=${version}\" -o ywai ./cmd/ywai"
    go build -ldflags="-X main.version=${version}" -o ywai ./cmd/ywai
    ok "Built ./ywai (version ${version})"
}

do_build_full() {
    local version
    version="$(compute_version)"
    info "Preparing embedded data..."
    cmd "bash scripts/prepare-embedded.sh"
    bash scripts/prepare-embedded.sh
    ok "Embedded data prepared"

    info "Building ywai WITH embedded data..."
    cmd "go build -tags embedded -ldflags=\"-X main.version=${version}\" -o ywai ./cmd/ywai"
    go build -tags embedded -ldflags="-X main.version=${version}" -o ywai ./cmd/ywai
    ok "Built ./ywai (version ${version}, embedded)"
}

do_install() {
    local version
    version="$(compute_version)"
    info "Full build + install to GOPATH..."

    info "Preparing embedded data..."
    cmd "bash scripts/prepare-embedded.sh"
    bash scripts/prepare-embedded.sh

    info "Installing..."
    cmd "go install -tags embedded -ldflags=\"-X main.version=${version}\" ./cmd/ywai"
    go install -tags embedded -ldflags="-X main.version=${version}" ./cmd/ywai

    local install_path
    install_path="$(command -v ywai 2>/dev/null || echo "${GOPATH:-$HOME/go}/bin/ywai")"
    ok "Installed ywai (version ${version}) at ${install_path}"
}

do_check() {
    local version
    version="$(compute_version)"
    info "=== Full pipeline check (version ${version}) ==="
    echo ""

    do_test

    echo ""
    do_build_full

    echo ""
    info "Verifying binary..."
    if [[ -f ./ywai ]]; then
        ./ywai version 2>/dev/null || ./ywai --version 2>/dev/null || ./ywai help 2>/dev/null || true
        local bin_version
        bin_version="$(./ywai version 2>/dev/null || true)"
        ok "Binary ./ywai exists and runs${bin_version:+ (${bin_version})}"
    else
        fail "Binary ./ywai not found after build"
        exit 1
    fi

    echo ""
    do_install

    echo ""
    ok "=== Full pipeline complete ==="
}

do_kanban() {
    local version
    version="$(compute_version)"
    info "Building + installing ywai with kanban support..."
    do_install

    echo ""
    info "Starting kanban daemon with UI on http://localhost:5768 ..."
    cmd "ywai serve --port 5768"
    ywai serve --port 5768
}

do_mcp_test() {
    local version
    version="$(compute_version)"
    info "Building + installing ywai (MCP test)..."
    do_install

    echo ""
    info "Verifying MCP daemon responds to initialize request..."

    # JSON-RPC initialize request
    local request
    request='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"ywai-dev-test","version":"1.0.0"}}}'

    cmd "echo '<json-rpc>' | ywai serve --mcp-only"
    local response
    response="$(printf '%s\n' "$request" | timeout 5 ywai serve --mcp-only 2>/dev/null || true)"

    if echo "$response" | grep -q '"jsonrpc":"2.0"' 2>/dev/null; then
        ok "MCP daemon responded with valid JSON-RPC"
        echo ""
        echo "$response" | python3 -m json.tool 2>/dev/null || echo "$response"
    else
        fail "MCP daemon did not return a valid JSON-RPC response"
        echo "Response was:"
        echo "$response"
        exit 1
    fi
}

do_version() {
    compute_version
}

do_help() {
    cat <<USAGE
ywai — Local development script

Usage:  ./scripts/dev.sh <subcommand>

Subcommands:

  test         Run all tests (go test ./... -v)
  test-kanban  Run only kanban tests
  build        Quick build WITHOUT embedded data (fast iteration)
  build-full   Full build WITH embedded data (prepare + -tags embedded)
  install      Full build + install to GOPATH
  check        Full pipeline: test → build-full → verify → install
  kanban       Build + install + start daemon with UI on port 5768
  mcp-test     Build + install + verify MCP daemon responds
  version      Print the version string that would be used
  help         Show this usage message

Run this script from within the ywai/ directory tree.
USAGE
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------
case "${1:-help}" in
    test)
        do_test
        ;;
    test-kanban)
        do_test_kanban
        ;;
    build)
        do_build
        ;;
    build-full)
        do_build_full
        ;;
    install)
        do_install
        ;;
    check)
        do_check
        ;;
    kanban)
        do_kanban
        ;;
    mcp-test)
        do_mcp_test
        ;;
    version)
        do_version
        ;;
    help|--help|-h)
        do_help
        ;;
    *)
        echo -e "${RED}Unknown subcommand: ${1}${NC}"
        echo ""
        do_help
        exit 1
        ;;
esac
