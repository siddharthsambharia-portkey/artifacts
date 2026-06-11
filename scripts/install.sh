#!/bin/sh
set -e

REPO="${ARTIFACT_REPO:-artifact/artifact}"
VERSION="${ARTIFACT_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep tag_name | cut -d'"' -f4)
fi

URL="https://github.com/${REPO}/releases/download/${VERSION}/artifact_${VERSION#v}_${OS}_${ARCH}.tar.gz"

echo "Installing Artifact ${VERSION} for ${OS}/${ARCH}..."

mkdir -p "$INSTALL_DIR"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if ! curl -fsSL "$URL" -o "$TMP/artifact.tar.gz" 2>/dev/null; then
  echo "Release binary not found. Building from source..."
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is required to build from source. Install Go 1.23+ and retry." >&2
    exit 1
  fi
  SRC=$(mktemp -d)
  git clone --depth 1 "https://github.com/${REPO}.git" "$SRC/artifact"
  (cd "$SRC/artifact" && go build -o "$INSTALL_DIR/artifact" ./cmd/artifact)
  rm -rf "$SRC"
else
  tar -xzf "$TMP/artifact.tar.gz" -C "$TMP"
  install -m 755 "$TMP/artifact" "$INSTALL_DIR/artifact"
fi

echo "✓ Installed to $INSTALL_DIR/artifact"
echo ""
echo "Quick start:"
echo "  artifact dev"
echo "  artifact init my-site && cd my-site && artifact deploy"

if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
  echo ""
  echo "Add to PATH: export PATH=\"$INSTALL_DIR:\$PATH\""
fi
