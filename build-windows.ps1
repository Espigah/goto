# Build script for goto on Windows
# Usage:
#   .\build-windows.ps1           # Build without voice (no CGO)
#   .\build-windows.ps1 -voice    # Build with voice support (requires w64devkit)

param(
    [switch]$voice = $false,
    [string]$w64devkit = "$env:USERPROFILE\Downloads\w64devkit"
)

$ErrorActionPreference = "Stop"

Write-Host "goto Windows Build Script" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan
Write-Host ""

# Check if Go is installed
try {
    $goVersion = & go version 2>&1
    Write-Host "Go found: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "Go not found. Please install Go 1.25.5+ from https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

if ($voice) {
    Write-Host ""
    Write-Host "Building with voice support (full build)..." -ForegroundColor Yellow
    
    # Check if w64devkit exists
    if (-not (Test-Path "$w64devkit\bin\gcc.exe")) {
        Write-Host "w64devkit not found at: $w64devkit" -ForegroundColor Red
        Write-Host "  Please install w64devkit or specify the path:" -ForegroundColor Red
        Write-Host "  .\build-windows.ps1 -voice -w64devkit C:\path\to\w64devkit" -ForegroundColor Red
        exit 1
    }
    
    Write-Host "w64devkit found at: $w64devkit" -ForegroundColor Green
    
    # Add w64devkit to PATH
    $env:PATH = "$w64devkit\bin;$env:PATH"
    $env:CGO_ENABLED = "1"

    # Build the GCC wrapper that converts pe-bigobj -> pe-x86-64 on the fly.
    # Required because GCC 16 (w64devkit) emits pe-bigobj-x86-64 by default,
    # which Go's CGO parser cannot read.
    Write-Host "Building GCC wrapper (pe-bigobj fix)..." -ForegroundColor Yellow
    $wrapSrc = Join-Path $PSScriptRoot "gcc_wrap.go"
    $wrapExe = Join-Path $PSScriptRoot "gcc_wrap.exe"
    $env:CGO_ENABLED = "0"   # build the wrapper itself without CGO
    & go build -tags ignore -o $wrapExe $wrapSrc
    if ($LASTEXITCODE -ne 0) {
        # fallback: build without tags (//go:build ignore prevents it from
        # being part of the main package, but we can build it explicitly)
        & go build -o $wrapExe $wrapSrc 2>$null
    }
    $env:CGO_ENABLED = "1"
    $env:CC = $wrapExe
    Write-Host "GCC wrapper ready: $wrapExe" -ForegroundColor Green
    
    # Set C/C++ paths for whisper.cpp
    $whisperDir = Join-Path $PSScriptRoot "third_party\whisper.cpp"
    $env:C_INCLUDE_PATH = "$whisperDir\include;$whisperDir\ggml\include"
    $env:LIBRARY_PATH = "$whisperDir\build_go\src;$whisperDir\build_go\ggml\src"
    
    Write-Host ""
    Write-Host "Building libwhisper (this may take a few minutes)..." -ForegroundColor Yellow
    
    Push-Location "$whisperDir\bindings\go"
    try {
        & make whisper
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to build libwhisper"
        }
        Write-Host "libwhisper built successfully" -ForegroundColor Green
    } finally {
        Pop-Location
    }
    
    Write-Host ""
    Write-Host "Building goto.exe with voice support..." -ForegroundColor Yellow
    $env:CGO_CFLAGS  = "-I$whisperDir\include -I$whisperDir\ggml\include"
    $env:CGO_LDFLAGS = "-L$whisperDir\build_go\src -L$whisperDir\build_go\ggml\src -lwhisper -lggml -lggml-base -lggml-cpu -lgomp -lstdc++ -lm"
    & go build -tags whisper -o goto.exe .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build successful!" -ForegroundColor Green
        Write-Host ""
        Write-Host "Voice-enabled goto.exe created (includes Whisper transcription)" -ForegroundColor Green
    } else {
        Write-Host "Build failed" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host ""
    Write-Host "Building without voice support (audio only via malgo/WASAPI)..." -ForegroundColor Yellow

    # Even the no-voice build uses CGO (malgo/WASAPI). Use the wrapper if w64devkit is present.
    $wrapExe = Join-Path $PSScriptRoot "gcc_wrap.exe"
    if (Test-Path "$w64devkit\bin\gcc.exe") {
        $env:PATH = "$w64devkit\bin;$env:PATH"
        $env:CGO_ENABLED = "1"
        if (-not (Test-Path $wrapExe)) {
            $env:CGO_ENABLED = "0"
            & go build -o $wrapExe (Join-Path $PSScriptRoot "gcc_wrap.go") 2>$null
            $env:CGO_ENABLED = "1"
        }
        $env:CC = $wrapExe
        Write-Host "Using GCC wrapper for CGO compatibility" -ForegroundColor Green
    }

    & go build -o goto.exe .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build successful!" -ForegroundColor Green
        Write-Host ""
        Write-Host "goto.exe created (mic + translate, no Whisper model)" -ForegroundColor Green
    } else {
        Write-Host "Build failed" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "Test the build:" -ForegroundColor Cyan
Write-Host "  .\goto.exe version" -ForegroundColor Gray
Write-Host "  .\goto.exe --help" -ForegroundColor Gray
Write-Host "  .\goto.exe list" -ForegroundColor Gray
Write-Host ""
