# ClawDaemon Windows Installer (PowerShell)
# Run as Administrator: iwr https://clawdaemon.dev/install.ps1 | iex

$repo = "https://github.com/yourusername/clawdaemon/releases/latest/download"
$binary = "clawdaemon-windows-amd64.exe"
$installDir = "$env:ProgramFiles\ClawDaemon"

Write-Host "🦀 Installing ClawDaemon..."
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Invoke-WebRequest -Uri "$repo/$binary" -OutFile "$installDir\clawdaemon.exe"

# Add to PATH
$currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
if ($currentPath -notlike "*ClawDaemon*") {
    [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$installDir", "Machine")
}

# Install as Windows Service
New-Service -Name "ClawDaemon" `
    -DisplayName "ClawDaemon - AI Agent Orchestrator" `
    -BinaryPathName "$installDir\clawdaemon.exe" `
    -StartupType Automatic

Write-Host "✅ Installed! Run: clawdaemon --wizard"
Write-Host "Start service: Start-Service ClawDaemon"
Write-Host "Dashboard: http://localhost:8080"
