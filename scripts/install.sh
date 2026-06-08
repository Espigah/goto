#!/usr/bin/env bash
# Install goto for the current user (no root) and set up autostart on login.
#   bin   -> ~/.local/bin/goto
#   icon  -> ~/.local/share/icons/hicolor/256x256/apps/goto.png
#   menu  -> ~/.local/share/applications/goto.desktop
#   login -> ~/.config/autostart/goto.desktop   (starts paused, see --paused)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BIN="$HOME/.local/bin/goto"
ICON="$HOME/.local/share/icons/hicolor/256x256/apps/goto.png"
APP_DESKTOP="$HOME/.local/share/applications/goto.desktop"
AUTOSTART="$HOME/.config/autostart/goto.desktop"

if [ ! -f "$ROOT/goto" ]; then
  echo "binary ./goto not found. Run 'make build-voice' first." >&2
  exit 1
fi

echo ">> binary   -> $BIN"
install -Dm755 "$ROOT/goto" "$BIN"
echo ">> icon     -> $ICON"
# An app must NEVER own the shared hicolor index. A per-user index.theme lives
# under $XDG_DATA_HOME, which outranks /usr/share/icons/hicolor in icon lookup;
# a minimal one (Directories=256x256/apps) shadows the system theme and breaks
# icon resolution desktop-wide. So register the icon with xdg-icon-resource, which
# never touches the shared index, and only fall back to dropping the PNG into
# 256x256/apps when that tool is missing. We never write an index.theme and never
# run gtk-update-icon-cache on the user's hicolor dir.
if command -v xdg-icon-resource >/dev/null 2>&1; then
  xdg-icon-resource install --novendor --size 256 "$ROOT/packaging/icons/goto.png" goto >/dev/null 2>&1 || true
else
  install -Dm644 "$ROOT/packaging/icons/goto.png" "$ICON"
fi
# Conservative self-heal: installers <=0.3.23 wrote a stub index.theme (and its
# icon-theme.cache) that caused exactly this breakage. Remove them only when the
# index.theme matches that single-directory signature, so a legitimate user theme
# is never touched.
HICOLOR="$HOME/.local/share/icons/hicolor"
if [ -f "$HICOLOR/index.theme" ] && grep -qx 'Directories=256x256/apps' "$HICOLOR/index.theme" 2>/dev/null; then
  rm -f "$HICOLOR/index.theme" "$HICOLOR/icon-theme.cache"
fi

# app-menu desktop entry (Exec without flag = auto-listen when you open it)
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
Keywords=voice;window;focus;
StartupWMClass=goto
EOF
echo ">> menu     -> $APP_DESKTOP"

# autostart on login: starts PAUSED (mic does not turn on by itself at boot)
install -d "$(dirname "$AUTOSTART")"
cat > "$AUTOSTART" <<EOF
[Desktop Entry]
Type=Application
Name=goto
Comment=Voice window control
Exec=$BIN --paused
Icon=goto
Terminal=false
StartupWMClass=goto
X-GNOME-Autostart-enabled=true
X-GNOME-Autostart-Delay=3
EOF
echo ">> autostart-> $AUTOSTART (starts paused)"

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" >/dev/null 2>&1 || true

echo ""
echo "OK. goto installed and will start on next login (in the tray, paused)."

# Launch now so it shows up in the tray immediately (detached via setsid so it
# survives this shell exiting).
if [ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]; then
  pkill -x goto >/dev/null 2>&1 || true
  sleep 0.3
  if command -v setsid >/dev/null 2>&1; then
    setsid "$BIN" >/dev/null 2>&1 < /dev/null &
  else
    nohup "$BIN" >/dev/null 2>&1 < /dev/null &
  fi
  disown >/dev/null 2>&1 || true
  sleep 1.5
  if pgrep -x goto >/dev/null 2>&1; then
    echo ">> goto is running (check your system tray)."
  else
    echo ">> goto installed but did not stay open. Start it with: $BIN"
  fi
else
  echo "Run now: $BIN"
fi
case ":$PATH:" in
  *":$HOME/.local/bin:"*) echo "Or just: goto" ;;
  *) echo "Tip: add ~/.local/bin to your PATH to call 'goto' directly." ;;
esac
