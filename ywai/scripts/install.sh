#!/usr/bin/env bash
set -euo pipefail

REPO="Yoizen/dev-ai-workflow"
BINARY="ywai"
VERSION="${1:-latest}"
INSTALL_DIR="${2:-/usr/local/bin}"
DATA_DIR="${HOME}/.ywai"

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

echo "  Cleaning old cached data..."
rm -rf "${DATA_DIR}/skills" "${DATA_DIR}/project-types"

echo "  Installing to ${INSTALL_DIR}..."
sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
sudo chmod +x "${INSTALL_DIR}/${BINARY}"

echo ""
echo "  ${BINARY} ${VERSION} installed!"
echo "  Run: ywai install"
