#!/usr/bin/env bash
set -euo pipefail

REPO="Yoizen/dev-ai-workflow"
BINARY="ywai"
VERSION="${1:-latest}"
INSTALL_DIR="${2:-${YWAI_INSTALL_DIR:-$HOME/.local/bin}}"
DATA_DIR="${HOME}/.ywai"

path_contains() {
    case ":${PATH}:" in
        *":$1:"*) return 0 ;;
        *) return 1 ;;
    esac
}

remove_old_ywai_binaries() {
    local final_path="$1"
    local -a candidates=(
        "$HOME/.local/bin/${BINARY}"
        "$HOME/bin/${BINARY}"
        "/usr/local/bin/${BINARY}"
        "/usr/bin/${BINARY}"
    )

    # Also include every resolved ywai in PATH (if available)
    if command -v which >/dev/null 2>&1; then
        while IFS= read -r p; do
            [ -n "$p" ] && candidates+=("$p")
        done < <(which -a "$BINARY" 2>/dev/null | awk '!seen[$0]++')
    fi

    local seen=""
    for old in "${candidates[@]}"; do
        [ -z "$old" ] && continue
        case " $seen " in *" $old "*) continue ;; esac
        seen="$seen $old"

        [ "$old" = "$final_path" ] && continue
        [ -e "$old" ] || continue

        if [ -w "$old" ] || [ -w "$(dirname "$old")" ]; then
            rm -f "$old" || true
            continue
        fi

        if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
            sudo rm -f "$old" || true
        fi
    done
}

install_binary() {
    local src="$1"

    mkdir -p "$INSTALL_DIR" 2>/dev/null || true

    if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
        install -m 755 "$src" "${INSTALL_DIR}/${BINARY}"
        return
    fi

    if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
        sudo mkdir -p "$INSTALL_DIR"
        sudo install -m 755 "$src" "${INSTALL_DIR}/${BINARY}"
        return
    fi

    local fallback="${HOME}/.local/bin"
    if [ "$INSTALL_DIR" != "$fallback" ]; then
        echo "  No write access to ${INSTALL_DIR} and sudo is not available without a cached password."
        echo "  Falling back to ${fallback}..."
        INSTALL_DIR="$fallback"
        mkdir -p "$INSTALL_DIR"
        install -m 755 "$src" "${INSTALL_DIR}/${BINARY}"
        return
    fi

    echo "Error: cannot write to ${INSTALL_DIR}."
    echo "Pass a writable install directory as the second argument, e.g.:"
    echo "  curl -fsSL https://github.com/${REPO}/releases/latest/download/install.sh | bash -s -- latest /some/bin"
    exit 1
}

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
fi

VERSION_CLEAN="${VERSION#v}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if [ "$OS" = "darwin" ]; then
    EXT="tar.gz"
elif [ "$OS" = "linux" ]; then
    EXT="tar.gz"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

FILENAME="${BINARY}_${VERSION_CLEAN}_${OS}_${ARCH}.${EXT}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Installing ${BINARY} ${VERSION}..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "  Downloading ${DOWNLOAD_URL}..."
curl -fsSL "$DOWNLOAD_URL" -o "${TMPDIR}/${FILENAME}"

echo "  Extracting..."
tar xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

echo "  Installing to ${INSTALL_DIR}..."
install_binary "${TMPDIR}/${BINARY}"

INSTALL_DIR="$(cd "$INSTALL_DIR" && pwd -P)"
INSTALL_PATH="${INSTALL_DIR}/${BINARY}"

echo "  Removing old ywai binaries from common paths..."
remove_old_ywai_binaries "$INSTALL_PATH"

echo "  Cleaning old cached data..."
rm -rf "${DATA_DIR}/skills" "${DATA_DIR}/project-types"

echo "  Seeding data..."
if ! "${INSTALL_PATH}" skills >/dev/null 2>&1; then
    echo "  Warning: data seed check failed. Try running: ${INSTALL_PATH} skills"
fi

echo ""
echo "  ${BINARY} ${VERSION} installed!"
echo "  Location: ${INSTALL_PATH}"

RUN_CMD="ywai install"

if ! path_contains "$INSTALL_DIR"; then
    echo ""
    echo "  Note: ${INSTALL_DIR} is not in your PATH."
    echo "  Add it with:"
    echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
    RUN_CMD="${INSTALL_PATH} install"
fi

ACTIVE_PATH="$(command -v "$BINARY" 2>/dev/null || true)"
if [ -n "$ACTIVE_PATH" ] && [ "$ACTIVE_PATH" != "$INSTALL_PATH" ]; then
    echo ""
    echo "  Warning: your shell currently resolves '${BINARY}' to:"
    echo "    ${ACTIVE_PATH}"
    echo "  Start a new terminal, run 'hash -r', or move ${INSTALL_DIR} earlier in PATH."
    RUN_CMD="${INSTALL_PATH} install"
fi

echo "  Run: ${RUN_CMD}"
