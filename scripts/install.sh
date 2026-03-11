#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-j1shnu/k8sweep}"
BIN_NAME="k8sweep"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
linux) OS="linux" ;;
darwin) OS="darwin" ;;
*)
	echo "Unsupported OS: $OS"
	exit 1
	;;
esac

case "$ARCH" in
x86_64 | amd64) ARCH="amd64" ;;
arm64 | aarch64) ARCH="arm64" ;;
*)
	echo "Unsupported architecture: $ARCH"
	exit 1
	;;
esac

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ARCHIVE="${BIN_NAME}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${ARCHIVE}"

echo "Downloading ${URL}"
curl -fsSL "$URL" -o "${TMP_DIR}/${ARCHIVE}"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"

mkdir -p "$INSTALL_DIR" 2>/dev/null || true
TARGET="${INSTALL_DIR}/${BIN_NAME}"

if [ -w "$INSTALL_DIR" ]; then
	install -m 0755 "${TMP_DIR}/${BIN_NAME}" "$TARGET"
else
	echo "Install dir is not writable: ${INSTALL_DIR}"
	echo "Trying sudo install to ${TARGET}"
	sudo install -m 0755 "${TMP_DIR}/${BIN_NAME}" "$TARGET"
fi

echo "Installed ${BIN_NAME} to ${TARGET}"
"${TARGET}" --version || true
