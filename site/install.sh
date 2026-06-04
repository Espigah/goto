#!/usr/bin/env bash
# goto installer.
#   curl -fsSL https://espigah.github.io/goto/install.sh | bash
#
# Downloads the latest goto release, installs it to ~/.local/bin (no root),
# sets up the system-tray app and login autostart (paused).
set -euo pipefail

REPO="Espigah/goto"
BIN="$HOME/.local/bin/goto"
ICON="$HOME/.local/share/icons/hicolor/256x256/apps/goto.png"
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

# app icon (for the desktop entry / menu)
curl -fsSL "https://raw.githubusercontent.com/$REPO/main/packaging/icons/goto.png" -o "$tmp/goto.png" 2>/dev/null \
  && install -Dm644 "$tmp/goto.png" "$ICON" || true

# app-menu entry (opens and auto-listens)
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
X-GNOME-Autostart-enabled=true
X-GNOME-Autostart-Delay=3
EOF

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" >/dev/null 2>&1 || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f "$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true

echo ""
echo "goto v$ver installed to $BIN"
case ":$PATH:" in *":$HOME/.local/bin:"*) : ;; *) echo "note: add ~/.local/bin to your PATH to run 'goto' directly." ;; esac

# launch now if in a graphical session
if [ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ] && ! pgrep -x goto >/dev/null 2>&1; then
  ( setsid "$BIN" >/dev/null 2>&1 < /dev/null & ) || ( "$BIN" >/dev/null 2>&1 & )
  echo "goto is starting in your system tray, and will start on next login too."
else
  echo "run it with: goto"
fi
echo "tip: for automatic updates, use the apt/dnf repo (see the README)."
