#!/usr/bin/env bash
# goto installer.
#   curl -fsSL https://espigah.github.io/goto/install.sh | bash
#
# Downloads the latest goto release, installs it to ~/.local/bin (no root),
# sets up the system-tray app and login autostart (paused), and launches it.
set -euo pipefail

REPO="Espigah/goto"
BIN="$HOME/.local/bin/goto"
ICONDIR="$HOME/.local/share/icons/hicolor"
APP_DESKTOP="$HOME/.local/share/applications/goto.desktop"
AUTOSTART="$HOME/.config/autostart/goto.desktop"

[ "$(uname -s)" = "Linux" ] || { echo "goto currently supports Linux only."; exit 1; }
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  *) echo "Unsupported architecture: $(uname -m) (only linux/amd64 for now)."; exit 1 ;;
esac
for t in curl tar; do command -v "$t" >/dev/null 2>&1 || { echo "missing required tool: $t"; exit 1; }; done

echo ">> finding the latest release"
ver="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -nE 's/.*"tag_name"[: ]+"v?([^"]+)".*/\1/p' | head -1)"
[ -n "$ver" ] || { echo "could not determine the latest version"; exit 1; }
echo ">> goto v$ver (linux/$ARCH)"

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
base="https://github.com/$REPO/releases/download/v$ver"
curl -fsSL "$base/goto_${ver}_linux_${ARCH}.tar.gz" -o "$tmp/goto.tgz"
tar xzf "$tmp/goto.tgz" -C "$tmp"
install -Dm755 "$tmp/goto" "$BIN"

# app icon: prefer the one bundled in the tarball, otherwise fetch from the repo.
icon_src="$tmp/goto.png"
[ -f "$icon_src" ] || curl -fsSL "https://raw.githubusercontent.com/$REPO/main/packaging/icons/goto.png" -o "$icon_src" 2>/dev/null || true
if [ -f "$icon_src" ]; then
  # An app must NEVER own the shared hicolor index. A per-user index.theme lives
  # under $XDG_DATA_HOME, which outranks /usr/share/icons/hicolor in icon lookup;
  # a minimal one (Directories=256x256/apps) shadows the system theme and breaks
  # icon resolution desktop-wide. So register the icon with xdg-icon-resource,
  # which never touches the shared index, and only fall back to dropping the PNG
  # into 256x256/apps when that tool is missing. We never write an index.theme and
  # never run gtk-update-icon-cache on the user's hicolor dir.
  if command -v xdg-icon-resource >/dev/null 2>&1; then
    xdg-icon-resource install --novendor --size 256 "$icon_src" goto >/dev/null 2>&1 || true
  else
    install -Dm644 "$icon_src" "$ICONDIR/256x256/apps/goto.png"
  fi
  # Conservative self-heal: installers <=0.3.23 wrote a stub index.theme (and its
  # icon-theme.cache) that caused exactly this breakage. Remove them only when the
  # index.theme matches that single-directory signature, so a legitimate user theme
  # is never touched.
  if [ -f "$ICONDIR/index.theme" ] && grep -qx 'Directories=256x256/apps' "$ICONDIR/index.theme" 2>/dev/null; then
    rm -f "$ICONDIR/index.theme" "$ICONDIR/icon-theme.cache"
  fi
fi

# app-menu entry (opens and auto-listens). StartupWMClass helps the desktop
# match the app to this icon.
install -d "$(dirname "$APP_DESKTOP")"
cat > "$APP_DESKTOP" <<EOF
[Desktop Entry]
Type=Application
Name=goto
GenericName=Voice window control
Comment=Focus windows/tabs by voice
Exec=$BIN
Icon=goto
Terminal=false
Categories=Utility;Accessibility;
StartupWMClass=goto
EOF

# autostart on login (starts paused: the mic does not turn on at boot)
install -d "$(dirname "$AUTOSTART")"
cat > "$AUTOSTART" <<EOF
[Desktop Entry]
Type=Application
Name=goto
Exec=$BIN --paused
Icon=goto
Terminal=false
StartupWMClass=goto
X-GNOME-Autostart-enabled=true
X-GNOME-Autostart-Delay=3
EOF

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" >/dev/null 2>&1 || true

echo ""
echo "goto v$ver installed to $BIN"
case ":$PATH:" in *":$HOME/.local/bin:"*) : ;; *) echo "note: add ~/.local/bin to your PATH to run 'goto' directly." ;; esac

# launch now if in a graphical session. setsid detaches it into its own session
# so it survives this shell exiting (important when run via `curl ... | bash`).
if [ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]; then
  pkill -x goto >/dev/null 2>&1 || true
  sleep 0.3
  if command -v setsid >/dev/null 2>&1; then
    setsid "$BIN" >/dev/null 2>&1 < /dev/null &
  else
    nohup "$BIN" >/dev/null 2>&1 < /dev/null &
  fi
  disown >/dev/null 2>&1 || true
  # verify it actually stayed up before claiming success
  sleep 1.5
  if pgrep -x goto >/dev/null 2>&1; then
    echo "goto is running now (look for it in the system tray); it also starts on next login."
  else
    echo "goto was installed but did not stay open. Start it yourself with: goto"
  fi
else
  echo "no graphical session detected. Start it with: goto"
fi
echo "tip: if the menu icon still looks generic, log out and back in (the desktop caches app icons)."
echo "tip: on GNOME, the tray icon needs the 'AppIndicator and KStatusNotifier Support' extension."
echo "tip: for automatic updates, use the apt/dnf repo (see the README)."
