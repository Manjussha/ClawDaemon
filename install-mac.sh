#!/bin/bash
# ClawDaemon macOS Installer
set -e
REPO="https://github.com/yourusername/clawdaemon/releases/latest/download"
ARCH=$(uname -m)
[ "$ARCH" = "arm64" ] && BINARY="clawdaemon-darwin-arm64" || BINARY="clawdaemon-darwin-amd64"
echo "🦀 Installing ClawDaemon ($BINARY)..."
curl -sSL "$REPO/$BINARY" -o /tmp/clawdaemon && chmod +x /tmp/clawdaemon
sudo mv /tmp/clawdaemon /usr/local/bin/clawdaemon
# Create launchd service
mkdir -p ~/Library/LaunchAgents
cat > ~/Library/LaunchAgents/com.clawdaemon.plist << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.clawdaemon</string>
  <key>ProgramArguments</key><array><string>/usr/local/bin/clawdaemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict></plist>
PLIST
echo "✅ Installed. Run: clawdaemon --wizard"
echo "Start: launchctl load ~/Library/LaunchAgents/com.clawdaemon.plist"
