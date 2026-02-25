#!/bin/sh
# ClawDaemon installer — Linux & macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/Manjussha/ClawDaemon/main/scripts/install.sh | sh
set -e

REPO="Manjussha/ClawDaemon"
BINARY="clawdaemon"
INSTALL_DIR="$HOME/.local/bin"

# ── Detect OS and architecture ────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)         ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  armv7l)         ARCH="arm"   ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux|darwin) ;;
  *)
    echo "Unsupported OS: $OS — use install.ps1 on Windows"
    exit 1
    ;;
esac

FILENAME="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}"

# ── Install ───────────────────────────────────────────────────────────────────
echo ""
echo "  ClawDaemon Installer"
echo "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  OS:   $OS"
echo "  Arch: $ARCH"
echo "  Dest: $INSTALL_DIR/$BINARY"
echo ""

mkdir -p "$INSTALL_DIR"

echo "  Downloading..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$INSTALL_DIR/$BINARY"
elif command -v wget >/dev/null 2>&1; then
  wget -q "$URL" -O "$INSTALL_DIR/$BINARY"
else
  echo "  Error: curl or wget is required."
  exit 1
fi

chmod +x "$INSTALL_DIR/$BINARY"
echo "  Installed: $INSTALL_DIR/$BINARY"

# ── PATH hint ─────────────────────────────────────────────────────────────────
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "  Add to PATH (add to ~/.bashrc or ~/.zshrc):"
    echo '    export PATH="$HOME/.local/bin:$PATH"'
    ;;
esac

echo ""
echo "  ✓ Done! Next steps:"
echo ""
echo "    clawdaemon setup   — run the setup wizard"
echo "    clawdaemon         — start with built-in defaults"
echo ""
