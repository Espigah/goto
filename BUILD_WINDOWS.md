# Building goto on Windows

This guide explains how to build goto on Windows without impacting the Linux build.

## Prerequisites

1. **Go 1.25.5+** - Download from https://go.dev/dl/
2. **w64devkit** (for voice build) - MinGW-w64 GCC toolchain with cmake

## Build Options

### Option 1: Window Focus Only (No Voice) - Recommended for Windows

This build works without CGO and doesn't require heavy dependencies:

```cmd
go build -tags noaudio -o goto.exe .
```

This creates a lightweight executable that supports:
- CLI window focusing: `goto vscode myproject`
- Window management via MCP for AI agents
- All window focus features

**Does NOT include:**
- Voice recognition (no "goto vscode" spoken commands)
- Microphone capture
- Whisper transcription

### Option 2: Full Build with Voice (Requires CGO)

For the full voice-enabled build, you need:

1. **Install w64devkit** (if not already available)
   - Download from: https://github.com/skeeto/w64devkit/releases
   - Extract to a known location (e.g., `C:\w64devkit`)

2. **Build libwhisper** (one-time setup)
   ```cmd
   set PATH=C:\path\to\w64devkit\bin;%PATH%
   cd third_party\whisper.cpp\bindings\go
   make whisper
   cd ..\..\..\..
   ```

3. **Build goto with voice**
   ```cmd
   set PATH=C:\path\to\w64devkit\bin;%PATH%
   set CGO_ENABLED=1
   set C_INCLUDE_PATH=%CD%\third_party\whisper.cpp\include;%CD%\third_party\whisper.cpp\ggml\include
   set LIBRARY_PATH=%CD%\third_party\whisper.cpp\build_go\src;%CD%\third_party\whisper.cpp\build_go\ggml\src
   go build -tags whisper -o goto.exe .
   ```

## Testing the Build

```cmd
REM Check version
goto.exe version

REM Test window focus (works in both builds)
goto.exe vscode myproject

REM Show help
goto.exe --help
```

## Impact on Linux Build

The build system uses Go build tags to conditionally compile code:

- `//go:build noaudio` - Stub implementation (Windows default)
- `//go:build !noaudio` - Full malgo implementation (Linux default)
- `//go:build whisper` - Whisper transcription (optional on all platforms)
- `//go:build !whisper` - Stub transcription (no voice)

**The Linux Makefile and build process remain unchanged.** The default Linux build uses:
```bash
make build-voice  # Full voice with Whisper
```

This works because:
1. No `-tags noaudio` is specified, so `audio_all.go` is used (includes malgo)
2. The `-tags whisper` enables Whisper transcription
3. CGO is enabled by default on Linux

## Notes

- The `noaudio` tag is Windows-specific and should only be used for Windows builds
- The pre-compiled executables (`goto_*.exe`) in the releases are built with the full voice support
- For development on Windows, the `noaudio` build is faster and doesn't require CGO setup
