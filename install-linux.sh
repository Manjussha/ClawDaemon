#!/bin/bash
# ClawDaemon Linux Installer
set -e
REPO="https://github.com/yourusername/clawdaemon/releases/latest/download"
BINARY="clawdaemon-linux-amd64"
[ "$(uname -m)" = "aarch64" ] && BINARY="clawdaemon-linux-arm64"
echo "🦀 Installing ClawDaemon ($BINARY)..."
curl -sSL "$REPO/$BINARY" -o /tmp/clawdaemon && chmod +x /tmp/clawdaemon
sudo mv /tmp/clawdaemon /usr/local/bin/clawdaemon
echo "✅ Installed. Run: clawdaemon --wizard"
