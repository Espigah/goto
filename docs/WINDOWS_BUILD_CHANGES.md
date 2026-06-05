# Windows Build Changes Summary

This document summarizes the changes made to enable Windows builds without impacting the Linux build system.

## Changes Made

### 1. Build Tags System (Already Existed)
The project already had a build tag system in place:
- `internal/audio/audio_all.go` - Tagged with `//go:build !noaudio` (full malgo implementation)
- `internal/audio/audio_stub.go` - Tagged with `//go:build noaudio` (stub for no-audio builds)

### 2. New Documentation
- **BUILD_WINDOWS.md** - Complete Windows build guide
  - Explains both build options (with/without voice)
  - Prerequisites and setup instructions
  - Testing procedures

### 3. New Build Script
- **build-windows.ps1** - PowerShell build script
  - Simple usage: `.\build-windows.ps1` (no voice) or `.\build-windows.ps1 -voice`
  - Automatic detection of w64devkit
  - Color-coded output for better UX
  - Builds libwhisper automatically for voice builds

### 4. Updated Makefile
- Added `build-windows` target for Windows lightweight builds
- Added comments explaining Windows build process
- No changes to existing Linux targets

## Build Options

### Option 1: Lightweight (Recommended for Windows)
```bash
go build -tags noaudio -o goto.exe .
```
- No CGO required
- No voice/audio features
- Window focus and MCP features work perfectly
- Fast compilation

### Option 2: Full Build with Voice
```bash
# Requires w64devkit (MinGW-w64 + cmake)
$env:PATH = "C:\path\to\w64devkit\bin;$env:PATH"
$env:CGO_ENABLED = "1"
go build -tags whisper -o goto.exe .
```
- Requires CGO and C++ toolchain
- Full voice recognition with Whisper
- Slower compilation
- Larger binary

## Impact on Linux Build

**Zero impact.** The Linux build continues to work exactly as before:

```bash
make build-voice  # Full voice build on Linux
make build        # Window focus only on Linux
```

The build tags ensure:
- Linux defaults to `audio_all.go` (malgo support) when no `-tags noaudio` is specified
- Windows can opt-in to lightweight build with `-tags noaudio`
- Voice support (`-tags whisper`) is optional on both platforms

## Testing

The Windows build was tested successfully:

```powershell
PS> .\goto-windows.exe version
goto v0.3.16

PS> .\goto-windows.exe --help
goto v0.3.16 - Voice window control
[... help output ...]
```

## Files Modified/Created

### Created:
- `BUILD_WINDOWS.md` - Windows build documentation
- `build-windows.ps1` - Windows build script
- `WINDOWS_BUILD_CHANGES.md` - This file

### Modified:
- `Makefile` - Added `build-windows` target and comments

### Existing (No Changes):
- `internal/audio/audio_all.go` - Already had correct build tags
- `internal/audio/audio_stub.go` - Already had correct build tags
- All Linux-specific files - Unchanged

## Recommendations

1. **For Windows developers**: Use the lightweight build (`-tags noaudio`)
   - Fast compilation
   - No CGO setup required
   - Perfect for testing window focus features

2. **For Windows releases**: Use the full voice build if needed
   - Requires CI/CD setup with w64devkit
   - Provides full feature parity with Linux

3. **For Linux**: No changes needed, existing build system works as-is

## Future Improvements

- Add GitHub Actions workflow for Windows builds
- Pre-compile Windows binaries with voice support in releases
- Add Windows-specific tests for the noaudio stub
