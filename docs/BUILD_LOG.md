# Windows Build Log

Chronological record of every build attempt and outcome.
For the final working setup, see `BUILD_WINDOWS.md`.

## Environment

- OS: Windows 10 (win32)
- Go: 1.23.8 (`C:\Program Files\Go`)
- GCC: 16.1.0 via w64devkit (`%USERPROFILE%\Downloads\w64devkit`)

---

## Root cause (confirmed after Attempt 4)

GCC 16.1.0 emits **`pe-bigobj-x86-64`** for every `.o`, even trivial ones.
Go's CGO parser rejects it. This is not a Go version issue — 1.23.8 and 1.25.5
fail identically.

---

## Attempt 1 — Standard build with voice

```powershell
.\build-windows.ps1 -voice
```

**FAILED**: `cgo: cannot parse gcc output _cgo_.o as ELF, Mach-O, PE, XCOFF object`

---

## Attempt 2 — gccgo compiler

```
go build -compiler=gccgo ...
```

**FAILED**: `ambiguous import: found package goto in multiple modules`
gccgo has path resolution issues with a module named `goto`.

---

## Attempt 3 — Change Go version

User downgraded/upgraded Go. **FAILED** — same CGO error regardless of version.

---

## Attempt 4 — GCC wrapper v1 (incomplete)

Wrote `gcc_wrap.go`: after GCC runs, call `objcopy -O pe-x86-64` on the output.

**FAILED** — wrapper parsed `-o path.o` (space) but CGO passes `-opath.o`
(concatenated, no space). So `_cgo_.o` was never converted.

Diagnosis: added logging to wrapper and confirmed `_cgo_.o` was still `pe-bigobj`
after the wrapper ran.

---

## Attempt 5 — No-audio build

```powershell
go build -tags noaudio -o goto_noaudio.exe .
```

**SUCCESS** — confirmed the issue is strictly CGO + GCC 16.

---

## Attempt 6 — Go 1.23.8 in go.mod

Changed `go.mod` to `go 1.23.8`. **FAILED** — same error. Confirmed: not a
Go version problem.

---

## Attempt 7 — GCC wrapper v2 (SUCCESS)

Fixed wrapper to detect `-opath.o` (concatenated) form.

Additional fixes required before the build succeeded:
- `gcc_wrap.go` and `cgotest.go` had `package main` + `func main()` conflicting
  with `main.go`. Fixed with `//go:build ignore`.
- `test.c` in repo root was picked up as a CGO source. Moved to `scripts/`.
- `mcp-go v0.54.1` declared `go 1.25.5` in its own `go.mod`, blocking 1.23.8.
  Downgraded to `v0.48.0` (identical API, compatible with Go 1.23).

```powershell
$env:CGO_ENABLED = "0"; go build -o gcc_wrap.exe gcc_wrap.go
$env:CGO_ENABLED = "1"; $env:CC = "$PWD\gcc_wrap.exe"
go build -o goto.exe .
```

**SUCCESS**: `goto.exe version` → `goto v0.3.19`, `goto.exe calibrate` opens mic.

---

## Attempt 8 — Voice build (SUCCESS)

`libwhisper.a` / `libggml*.a` were already compiled. Link failed:

```
undefined reference to `GOMP_parallel'
undefined reference to `GOMP_barrier'
```

Cause: ggml compiled with `-fopenmp` but `-lgomp` not passed to final link.
Fix: add `-lgomp` to `CGO_LDFLAGS`. `libgomp.a` is at
`w64devkit\lib\gcc\x86_64-w64-mingw32\16.1.0\libgomp.a`.

```powershell
$env:CGO_LDFLAGS = "-L...\build_go\src -L...\build_go\ggml\src -lwhisper -lggml -lggml-base -lggml-cpu -lgomp -lstdc++ -lm"
go build -tags whisper -o goto.exe .
```

**SUCCESS**: voice-enabled `goto.exe` installed and running in system tray.

---

## Post-build runtime fixes

### Explorer not focusing (`nothing matched [explorer]`)

Windows Explorer class is `CabinetWClass`, not `explorer`.
Fixed in `internal/adapter/builtin.go`:
```go
Simple([]string{"explorador", "explorer", ...}, []string{"nautilus", "cabinetWClass"})
```

### Chrome adapter focusing Kiro/Electron apps

`Chrome_WidgetWin_1` is the window class for all Electron apps (Kiro, VS Code,
Slack, Discord), not just Chrome. Fixed `internal/adapter/chrome.go` to filter
by title containing `"google chrome"`.

### Wake word not detected ("Gochukiro", "Gocho Quiro", "Cocho crome")

Whisper PT fuses wake+command into one token or swaps G→C.
Fixes in `internal/wake/wake.go`:
- Added split logic: try every prefix of length 4–8 against the variants list
- Added PT variants: `gocho`, `gochua`, `gochu`, `cocho`, `coto`, `coche`
- `mcp-go` version note: `v0.48` is the last version compatible with Go 1.23

### Language default changed to `"pt"`

`internal/config/config.go`: `Language` default changed from `"en"` to `"pt"`
for better accuracy on Portuguese speakers.
`biasPrompt` updated to include PT app names (`navegador`, `explorador`).
