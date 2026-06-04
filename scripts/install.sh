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
install -Dm644 "$ROOT/packaging/icons/goto.png" "$ICON"

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
X-GNOME-Autostart-enabled=true
X-GNOME-Autostart-Delay=3
EOF
echo ">> autostart-> $AUTOSTART (starts paused)"

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" >/dev/null 2>&1 || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f "$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true

echo ""
echo "OK. goto installed and will start on next login (in the tray, paused)."

# Launch now so it shows up in the tray immediately (detached from the shell).
if [ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ] && ! pgrep -x goto >/dev/null 2>&1; then
  ( setsid "$BIN" >/dev/null 2>&1 < /dev/null & ) || ( "$BIN" >/dev/null 2>&1 & )
  echo ">> goto launched (check your system tray)."
else
  echo "Run now: $BIN"
fi
case ":$PATH:" in
  *":$HOME/.local/bin:"*) echo "Or just: goto" ;;
  *) echo "Tip: add ~/.local/bin to your PATH to call 'goto' directly." ;;
esac
