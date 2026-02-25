# ClawDaemon installer — Windows PowerShell
# Usage: iwr -useb https://raw.githubusercontent.com/yourusername/clawdaemon/main/scripts/install.ps1 | iex
#
# Run in PowerShell (not cmd):
#   Set-ExecutionPolicy RemoteSigned -Scope CurrentUser   # allow scripts once
#   iwr -useb https://raw.githubusercontent.com/.../install.ps1 | iex

$ErrorActionPreference = "Stop"

$REPO        = "yourusername/clawdaemon"
$BINARY      = "clawdaemon.exe"
$INSTALL_DIR = "$env:LOCALAPPDATA\Programs\ClawDaemon"

# ── Detect architecture ───────────────────────────────────────────────────────
$ARCH = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$FILENAME = "clawdaemon-windows-$ARCH.exe"
$URL = "https://github.com/$REPO/releases/latest/download/$FILENAME"

Write-Host ""
Write-Host "  ClawDaemon Installer" -ForegroundColor Cyan
Write-Host "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Host "  OS:   Windows"
Write-Host "  Arch: $ARCH"
Write-Host "  Dest: $INSTALL_DIR\$BINARY"
Write-Host ""

# ── Create install directory ──────────────────────────────────────────────────
New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null

# ── Download ──────────────────────────────────────────────────────────────────
Write-Host "  Downloading..."
try {
    Invoke-WebRequest -Uri $URL -OutFile "$INSTALL_DIR\$BINARY" -UseBasicParsing
} catch {
    Write-Host "  Error: $_" -ForegroundColor Red
    Write-Host "  Download manually from: https://github.com/$REPO/releases"
    exit 1
}
Write-Host "  Installed: $INSTALL_DIR\$BINARY"

# ── Add to user PATH ──────────────────────────────────────────────────────────
$currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($currentPath -notlike "*$INSTALL_DIR*") {
    [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$INSTALL_DIR", "User")
    Write-Host "  Added to PATH (restart terminal to take effect)"
}

Write-Host ""
Write-Host "  Done! Next steps:" -ForegroundColor Green
Write-Host ""
Write-Host "    clawdaemon setup   # run the setup wizard"
Write-Host "    clawdaemon         # start with built-in defaults"
Write-Host ""
