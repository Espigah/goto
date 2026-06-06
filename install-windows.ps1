# goto - Windows Installer Script
# Run this once to install goto from a GitHub release.
#
# Usage (run in PowerShell as normal user, no admin needed):
#   irm https://raw.githubusercontent.com/Espigah/goto/main/install-windows.ps1 | iex
#
# Or locally after cloning:
#   .\install-windows.ps1
#
# What it does:
#   1. Downloads the latest goto.exe from GitHub releases
#   2. Installs to %LOCALAPPDATA%\Programs\goto\
#   3. Adds that folder to the user PATH
#   4. Creates a Start Menu shortcut with icon
#   5. Launches goto (tray icon appears immediately)

$ErrorActionPreference = "Stop"

$repo    = "Espigah/goto"
$installDir = "$env:LOCALAPPDATA\Programs\goto"

Write-Host ""
Write-Host "goto installer" -ForegroundColor Cyan
Write-Host "==============" -ForegroundColor Cyan
Write-Host ""

# --- 1. Find latest release ---
Write-Host "Checking latest release..." -ForegroundColor Yellow
try {
    $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
    $version = $release.tag_name
    $asset   = $release.assets | Where-Object { $_.name -like "*windows_amd64.exe" } | Select-Object -First 1
    if (-not $asset) {
        # fallback: any .exe that is not the installer
        $asset = $release.assets | Where-Object { $_.name -like "goto_*.exe" -and $_.name -notlike "*installer*" } | Select-Object -First 1
    }
    if (-not $asset) { throw "No Windows executable found in release $version" }
    Write-Host "Latest: $version" -ForegroundColor Green
} catch {
    Write-Host "Could not reach GitHub API: $_" -ForegroundColor Red
    Write-Host "Download manually from: https://github.com/$repo/releases/latest" -ForegroundColor Yellow
    exit 1
}

# --- 2. Download ---
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
$exePath = "$installDir\goto.exe"

Write-Host "Downloading $($asset.name)..." -ForegroundColor Yellow
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $exePath -UseBasicParsing
Write-Host "Downloaded to: $exePath" -ForegroundColor Green

# --- 3. PATH ---
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$installDir;$userPath", "User")
    $env:PATH = "$installDir;$env:PATH"
    Write-Host "Added to PATH" -ForegroundColor Green
} else {
    Write-Host "Already in PATH" -ForegroundColor Gray
}

# --- 4. Start Menu shortcut ---
$shortcutPath = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\goto.lnk"
$wsh = New-Object -ComObject WScript.Shell
$sc  = $wsh.CreateShortcut($shortcutPath)
$sc.TargetPath       = $exePath
$sc.IconLocation     = "$exePath,0"
$sc.Description      = "Voice window control"
$sc.WorkingDirectory = $installDir
$sc.Save()
Write-Host "Start Menu shortcut created" -ForegroundColor Green

# --- 5. Launch ---
# Kill any running instance first
Get-Process goto -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 400
Start-Process $exePath -WindowStyle Hidden
Start-Sleep -Seconds 2

if (Get-Process goto -ErrorAction SilentlyContinue) {
    Write-Host ""
    Write-Host "goto $version is running." -ForegroundColor Green
    Write-Host "Look for the icon in the system tray (bottom-right)." -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Say 'goto chrome' or 'goto vscode' to focus a window." -ForegroundColor Cyan
    Write-Host "Right-click the tray icon to start listening or quit." -ForegroundColor Cyan
} else {
    Write-Host "goto started but process not detected. Check the tray manually." -ForegroundColor Yellow
}

Write-Host ""
