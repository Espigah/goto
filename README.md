<p align="center">
  <img src="site/logo.png" alt="goto" width="160">
</p>

<h1 align="center">goto</h1>

<p align="center">
  <a href="https://github.com/Espigah/goto/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/Espigah/goto/ci.yml?branch=main&label=build&color=6e9bff&labelColor=161b22" alt="build status"></a>
  <a href="https://github.com/Espigah/goto/releases/latest"><img src="https://img.shields.io/github/v/release/Espigah/goto?display_name=tag&label=release&color=6e9bff&labelColor=161b22" alt="latest release"></a>
  <a href="https://github.com/Espigah/goto/releases"><img src="https://img.shields.io/github/downloads/Espigah/goto/total?label=downloads&color=6e9bff&labelColor=161b22" alt="downloads"></a>
</p>

Hunting for a window lost among dozens of others, with the mouse and Alt+Tab,
is slow and annoying. **goto** fixes that: say where you want to go and the
window jumps to the front. On Linux, fully offline.

Voice control for windows. Say what you want to focus:

```
"goto vscode myproject"  ->  focus the VS Code window for that project
"goto slack john doe"    ->  open the DM with that person
"goto chrome github"     ->  switch to the right Chrome tab
"goto terminal logs"     ->  focus the right terminal
```

Transcription runs **100% locally** (Whisper via whisper.cpp). Nothing leaves
your machine. No account, no API key on the default path.

## How it works

```
microphone (malgo) -> VAD (drops silence) -> Whisper (speech->text, local)
   -> wake word "goto" (accent-tolerant) -> dispatcher -> focus the window
```

- **Per-program adapters** (`internal/adapter`): each app is an isolated plugin.
  Adding a new program is one file, no core changes. See [docs/ADAPTERS.md](docs/ADAPTERS.md).
- **Per-OS backends** (`winfocus.Backend`): X11 today; Wayland/Windows/macOS
  are just an interface to implement.
- **Accent-tolerant wake word**: "goto", "go to", "good to", "gotchu", and even
  when the recognizer eats the start, goto still understands you.

## Activation modes

| Mode | Account? | How |
|------|----------|-----|
| **hotkey** (default) | no | hold a key (e.g. `ctrl+alt+space`), speak, release |
| **wake word "goto"** | no | hands-free; say "goto ..." anytime |
| Porcupine | yes (free, BYOK) | opt-in, lower CPU when always-on (see [docs/PORCUPINE.md](docs/PORCUPINE.md)) |

Switch modes from the tray menu.

## What you can do

```
goto vscode backend       # focus the BACKEND project window
goto vscode app config    # open a file via Quick Open (Ctrl+P)
goto chrome github        # switch to the matching Chrome tab (Ctrl+Shift+A)
goto slack john doe       # open the Slack DM (Ctrl+K Jump to)
goto terminal logs        # focus the right terminal
goto browser              # focus the only open browser (does nothing if 2+)
```

## Install

### Quick install (one line)

```bash
curl -fsSL https://espigah.github.io/goto/install.sh | bash
```

Downloads the latest goto, installs it to `~/.local/bin`, and sets up the
system tray + login autostart. To update later, re-run the same line (or use
the apt/dnf repo below for automatic updates).

To uninstall (removes the binary, menu/autostart entries, icon, and the
downloaded voice model + config):

```bash
curl -fsSL https://espigah.github.io/goto/uninstall.sh | bash
```

If you installed from source, `make uninstall` does the same.

### Via apt/dnf repository (recommended, auto-updates)

Add the repo once; then `apt upgrade` / `dnf upgrade` (or the system's
automatic updater) keeps goto up to date.

```bash
# Debian / Ubuntu
curl -fsSL https://espigah.github.io/goto/apt/key.gpg | sudo tee /usr/share/keyrings/goto.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/goto.gpg] https://espigah.github.io/goto/apt ./" | sudo tee /etc/apt/sources.list.d/goto.list
sudo apt update && sudo apt install goto
```

```bash
# Fedora / RHEL
sudo tee /etc/yum.repos.d/goto.repo >/dev/null <<'EOF'
[goto]
name=goto
baseurl=https://espigah.github.io/goto/yum
enabled=1
repo_gpgcheck=1
gpgcheck=0
gpgkey=https://espigah.github.io/goto/yum/key.gpg
EOF
sudo dnf install goto
```

### Or download a single package

From the [Releases page](https://github.com/Espigah/goto/releases/latest):

```bash
# Debian / Ubuntu
sudo apt install ./goto_<version>_linux_amd64.deb

# Fedora / RHEL
sudo dnf install ./goto_<version>_linux_amd64.rpm

# Portable (tar.gz)
tar xzf goto_<version>_linux_amd64.tar.gz && ./goto
```

### AppImage (single portable file, no install)

One self-contained file that runs on most distros, it bundles the C/C++
runtime so you don't hit missing-library errors:

```bash
chmod +x goto_<version>_x86_64.AppImage
./goto_<version>_x86_64.AppImage
```

The Whisper model (~466MB) is downloaded on first use to
`~/.local/share/goto/models/`. goto runs in the system tray.

### Start with the system (autostart)

```bash
make install      # installs to ~/.local/bin + tray autostart on login (no root)
make uninstall    # removes it
```

Autostart launches goto **paused** (it does not turn the mic on at login).

### From source

Requirements: Go 1.25+, gcc/g++, cmake (build only).

```bash
git clone <repo> goto && cd goto
make lib            # build libwhisper once
make build-voice    # binary with offline voice
./goto              # tray app

# or window focus only, no voice (lighter):
make build && ./goto vscode myproject
```

## Use it as an arm of Claude Code (MCP)

goto exposes its desktop actions as MCP tools, so Claude Code can focus
windows, switch tabs and open conversations for you:

```bash
claude mcp add --transport stdio goto -- /path/to/goto mcp
```

Then ask, in plain language: "focus the myproject VS Code and open the github
tab in Chrome", and Claude calls goto's `list_windows` / `run` tools.

## Command line

Without voice, handy for scripts or to bind to an OS shortcut:

```bash
goto <app> [target]
goto vscode myproject
goto terminal logs
goto slack john doe
```

## Configuration

`~/.config/goto/config.json` (mode, model, language, hotkey, wake word).

## Development

```bash
make test        # pure-logic tests (wake, vad, dispatch, config)
make test-voice  # includes real transcription (needs lib + model)
make vet
go run ./cmd/winls   # list windows (backend debug)
```

Adapter contribution guide: [docs/ADAPTERS.md](docs/ADAPTERS.md).
Roadmap: [docs/ROADMAP.md](docs/ROADMAP.md).

## Privacy

Audio and transcription never leave your machine. The only network access is
the one-time Whisper model download on first use.

## License

MIT

---

**[Visit the landing page](https://espigah.github.io/goto/)**
