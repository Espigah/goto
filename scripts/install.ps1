# goto installation script for Windows
# Usage: .\install.ps1 [-autostart]

param(
    [switch]$autostart = $false
)

$ErrorActionPreference = "Stop"

Write-Host "goto Windows Installation" -ForegroundColor Cyan
Write-Host "========================" -ForegroundColor Cyan
Write-Host ""

# Check if goto.exe exists
if (-not (Test-Path "goto.exe")) {
    Write-Host "Error: goto.exe not found in current directory" -ForegroundColor Red
    Write-Host "Please run this script from the goto build directory" -ForegroundColor Red
    exit 1
}

# Test if the exe works
Write-Host "Testing goto.exe..." -ForegroundColor Yellow
$version = & .\goto.exe version 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "OK goto.exe is working: $version" -ForegroundColor Green
} else {
    Write-Host "FAIL goto.exe test failed" -ForegroundColor Red
    exit 1
}

# Create bin directory
$binDir = Join-Path $env:USERPROFILE "bin"
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
    Write-Host "OK Created directory $binDir" -ForegroundColor Green
} else {
    Write-Host "OK Directory $binDir exists" -ForegroundColor Green
}

# Copy executable
$exePath = Join-Path $binDir "goto.exe"
Copy-Item "goto.exe" $exePath -Force
Write-Host "OK Installed goto.exe to $binDir" -ForegroundColor Green

# Add to PATH if not present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$binDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$binDir", "User")
    $env:PATH = "$env:PATH;$binDir"
    Write-Host "OK Added $binDir to User PATH" -ForegroundColor Green
    Write-Host "   Note: New terminals will have goto in PATH automatically" -ForegroundColor Gray
} else {
    Write-Host "OK $binDir is already in PATH" -ForegroundColor Green
}

# Setup autostart if requested
if ($autostart) {
    Write-Host ""
    Write-Host "Setting up autostart..." -ForegroundColor Yellow
    
    try {
        & $exePath help | Out-Null
        Write-Host "OK Autostart will be configured on first run" -ForegroundColor Green
        Write-Host "   Run 'goto' and enable 'Start at login' from the tray menu" -ForegroundColor Gray
    } catch {
        Write-Host "! Autostart setup skipped - configure manually if needed" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Quick start:" -ForegroundColor Cyan
Write-Host "  goto version              Check installation" -ForegroundColor Gray
Write-Host "  goto --help               Show all commands" -ForegroundColor Gray
Write-Host "  goto vscode myproject     Focus VS Code window (CLI)" -ForegroundColor Gray
Write-Host "  goto                      Start tray app (GUI)" -ForegroundColor Gray
Write-Host ""

if (-not $autostart) {
    Write-Host "To enable autostart, run:" -ForegroundColor Cyan
    Write-Host "  .\install.ps1 -autostart" -ForegroundColor Gray
    Write-Host ""
}
