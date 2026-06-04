# goto, Roadmap

Voice control for windows, offline-first, cross-platform.

## Principles

- **Offline by default.** The main path depends on no external service or
  account. Transcription runs locally (Whisper). Cloud/3rd-party is opt-in.
- **Community-extensible.** Each program is an isolated adapter (see
  `docs/ADAPTERS.md`); each OS is an isolated backend (`winfocus.Backend`).
- **Easy to install.** Native packages for the most common managers.
- **Tested.** Every package with logic has unit tests. A new adapter ships
  with a test (synthetic windows in `internal/dispatch`). CI runs
  `go test ./...` + `go vet` on each PR. Pure logic (normalization, ranking,
  command parsing, VAD) is tested without touching audio/X.

## Milestones

| # | Item | Status |
|---|------|--------|
| M1 | Chassis: tray + on/off toggle + audio capture (malgo) | done |
| M4 | Window focus (EWMH X11) + dispatcher | done |
| M4.5 | Per-program adapters + contribution guide | done |
| M3 | Offline Whisper (transcription), `stt.Transcriber` interface | done |
| M2a | **hotkey / push-to-talk** activation (offline, default) | done |
| M2b | **Offline "goto" wake word** (VAD + Whisper), accent-tolerant | done |
| M2c | **Porcupine** wake word (Plan B, opt-in, BYOK Picovoice) | documented (docs/PORCUPINE.md) |
| Mc | MCP server (`goto mcp`): list_windows / run for Claude Code | done |
| M5 | Packaging + distribution (deb/rpm via GoReleaser; AppImage/snap configs) | deb/rpm/tar.gz shipping; AppImage/snap pending |
| M6 | Installer + autostart on login (with an opt-out toggle) | done |
| Mt | Unit tests per package + CI | done |

## Rich adapters (the "focus app + native shortcut + type" recipe)

- VS Code: open a file via Ctrl+P (Quick Open)
- Chrome: switch tab via Ctrl+Shift+A (tab search)
- Slack: open a DM/channel via Ctrl+K (Jump to)

Built on keyboard injection (XTEST) in `internal/keyinject`.

## Next

- AppImage / snap in the pipeline (today it ships deb/rpm/tar.gz).
- Voice -> Claude direction: a "claude ..." wake word that sends the local
  transcript to Claude (Agent SDK), which then acts through goto's MCP tools.
- Phonetic wake-word matching (instead of a curated variant list).
- Wayland / Windows / macOS backends.
