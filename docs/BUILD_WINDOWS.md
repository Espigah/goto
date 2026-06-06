# Windows Build Guide (for LLMs / contributors)

This document is the authoritative reference for building goto on Windows.
It is written for LLMs and developers, not end users.
End-user installation: download the installer from the GitHub releases page.

---

## Environment matrix

| Component | Value |
|-----------|-------|
| Go | 1.23.8 (installed at `C:\Program Files\Go`) |
| GCC | 16.1.0 via **w64devkit** (`%USERPROFILE%\Downloads\w64devkit`) |
| Shell | PowerShell (pwsh or the built-in Windows PowerShell 5) |
| CGO | Required for `malgo` (WASAPI mic) and `whisper.cpp` |

---

## The GCC 16 / CGO incompatibility (root cause)

GCC 16.1.0 (w64devkit) emits **`pe-bigobj-x86-64`** format for every `.o` file,
even trivially small ones. Go's CGO parser only accepts standard PE (COFF),
ELF, Mach-O, and XCOFF. It rejects `pe-bigobj` with:

```
cgo: cannot parse gcc output $WORK\b001\\_cgo_.o as ELF, Mach-O, PE, XCOFF object
```

This is **not** a Go version issue. Go 1.23.8 and 1.25.5 both fail identically.

### Fix: gcc_wrap.go

`gcc_wrap.go` (root of the repo, `//go:build ignore`) is a thin GCC wrapper:

1. Passes all arguments through to the real `gcc.exe`
2. After `gcc` succeeds, finds the output `.o` path — handling **both** forms:
   - `-o path.o`  (space-separated)
   - `-opath.o`   (concatenated — what CGO actually uses for `_cgo_.o`)
3. Runs `objcopy -O pe-x86-64 out.o tmp.o && mv tmp.o out.o`

Set `CC` to the wrapper before any CGO build:

```powershell
$env:CGO_ENABLED = "0"
go build -o gcc_wrap.exe gcc_wrap.go   # build the wrapper without CGO

$env:CGO_ENABLED = "1"
$env:CC = "$PWD\gcc_wrap.exe"
go build ...
```

The wrapper is in `.gitignore` (`gcc_wrap.exe`) — never commit the compiled binary.

---

## Build variants

### 1. No-audio (no CGO, fastest)

No GCC needed. Window focus and MCP only. No mic, no voice.

```powershell
go build -tags noaudio -o goto.exe .
```

### 2. Audio only (CGO via malgo/WASAPI, no Whisper)

Mic capture works. Voice STT stub — the tray shows "voice unavailable".

```powershell
$w64 = "$env:USERPROFILE\Downloads\w64devkit"
$env:PATH = "$w64\bin;$env:PATH"
$env:CGO_ENABLED = "0"; go build -o gcc_wrap.exe gcc_wrap.go
$env:CGO_ENABLED = "1"; $env:CC = "$PWD\gcc_wrap.exe"
go build -o goto.exe .
```

### 3. Full voice build (CGO + whisper.cpp) — what ships to users

Requires `libwhisper.a` to already be compiled (see below).

```powershell
$w64        = "$env:USERPROFILE\Downloads\w64devkit"
$whisperDir = Resolve-Path ".\third_party\whisper.cpp"

$env:PATH        = "$w64\bin;$env:PATH"
$env:CGO_ENABLED = "0"; go build -o gcc_wrap.exe gcc_wrap.go
$env:CGO_ENABLED = "1"
$env:CC          = "$PWD\gcc_wrap.exe"
$env:CGO_CFLAGS  = "-I$whisperDir\include -I$whisperDir\ggml\include"
$env:CGO_LDFLAGS = "-L$whisperDir\build_go\src -L$whisperDir\build_go\ggml\src -lwhisper -lggml -lggml-base -lggml-cpu -lgomp -lstdc++ -lm"

go build -tags whisper -o goto.exe .
```

> **`-lgomp` is required.** The whisper/ggml static libs are compiled with
> `-fopenmp`. Without `-lgomp` the link fails with
> `undefined reference to GOMP_parallel` / `GOMP_barrier`.

### Script shortcut

```powershell
.\build-windows.ps1           # variant 2 (audio, no voice)
.\build-windows.ps1 -voice    # variant 3 (full)
.\build-windows.ps1 -voice -w64devkit C:\tools\w64devkit  # custom path
```

---

## Building libwhisper (one-time per machine / CI cache)

```powershell
$w64 = "$env:USERPROFILE\Downloads\w64devkit"
$env:PATH = "$w64\bin;$env:PATH"
Push-Location "third_party\whisper.cpp\bindings\go"
make whisper
Pop-Location
```

Output: `third_party/whisper.cpp/build_go/src/libwhisper.a` + ggml libs.
This directory is in `.gitignore` — never commit build artifacts.

---

## CI pipeline (GitHub Actions)

`windows.yml` runs on `push: tags: v*` and `workflow_dispatch`.

Key steps:
1. **MSYS2 UCRT64** toolchain (not w64devkit) — GCC from pacman, same problem
   does NOT exist there because the CI uses a known-compatible GCC version.
2. **Lib cache** keyed on whisper source hash — avoids rebuilding on every run.
3. **Lib name normalization** — cmake on Windows produces `ggml.a` (no `lib`
   prefix), but `-lggml` needs `libggml.a`. The workflow copies each.
4. **`-lgomp` via `-fopenmp`** — `CGO_LDFLAGS="-fopenmp"` lets GCC pull in
   `libgomp` automatically. Equivalent to listing `-lgomp` explicitly.
5. **`-extldflags "-static"`** — links `libgcc`, `libstdc++`, `libwinpthread`,
   and `libgomp` statically so the `.exe` has no MinGW DLL dependencies.
6. **Inno Setup** builds the installer after the `.exe`.

The CI does **not** use `gcc_wrap.go` — the MSYS2 GCC does not emit `pe-bigobj`
and the wrapper is only needed for w64devkit GCC 16 on developer machines.

---

## Known issues / gotchas

| Symptom | Cause | Fix |
|---------|-------|-----|
| `cgo: cannot parse ... as ELF/PE` | GCC 16 emits `pe-bigobj` | Use `gcc_wrap.exe` as `CC` |
| `undefined reference to GOMP_parallel` | Missing `-lgomp` | Add `-lgomp` to `CGO_LDFLAGS` |
| `go: module requires go >= 1.25.5` | `mcp-go >= v0.49` bumped go version | Pin to `v0.48.0` |
| `package goto: C source files not allowed` | `test.c` in root picked up by Go | Moved to `scripts/` |
| `main redeclared` | `gcc_wrap.go`/`cgotest.go` lacked `//go:build ignore` | Tag added |
| Chrome adapter focuses Kiro/Electron apps | `Chrome_WidgetWin_1` shared by all Electron | Filter by `"Google Chrome"` in title |

---

## Files that must NOT be committed

See `.gitignore`. Key entries:
- `gcc_wrap.exe` — compiled locally, platform-specific
- `gcc_wrapper.bat` — superseded by `gcc_wrap.go`
- `goto.exe`, `goto_*.exe` — build artifacts
- `*.syso` — generated by `windres` in CI
- `third_party/whisper.cpp/build_go/` — compiled C libs
- `nm1.txt`, `nm2.txt`, `wrapper.log` — debug scratch files
