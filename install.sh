#!/bin/sh
set -e

REPO="ryomappi/kaiten"
BIN="kaiten"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to fetch the latest version." >&2
  exit 1
fi

ARCHIVE="${BIN}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

echo "Installing ${BIN} ${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" | tar -xz -C "$TMP"
mkdir -p "$INSTALL_DIR"
install -m 755 "$TMP/${BIN}" "${INSTALL_DIR}/${BIN}"

echo "Installed to ${INSTALL_DIR}/${BIN}"

case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "NOTE: Add ${INSTALL_DIR} to your PATH: export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
esac
