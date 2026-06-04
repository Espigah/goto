#!/usr/bin/env bash
# goto uninstaller.
#   curl -fsSL https://espigah.github.io/goto/uninstall.sh | bash
#
# Removes everything install.sh created: the binary, the menu and autostart
# entries, the icon, and the downloaded voice model + config. No root needed
# (it only touches files under your home directory).
set -euo pipefail

BIN="$HOME/.local/bin/goto"
ICON="$HOME/.local/share/icons/hicolor/256x256/apps/goto.png"
APP_DESKTOP="$HOME/.local/share/applications/goto.desktop"
AUTOSTART="$HOME/.config/autostart/goto.desktop"
DATA="$HOME/.local/share/goto"      # downloaded Whisper model (~466MB)
CONFIG="$HOME/.config/goto"         # config.json, aliases

echo ">> stopping goto"
pkill -x goto >/dev/null 2>&1 || true

# drop the icon registered via xdg-icon-resource (best effort)
command -v xdg-icon-resource >/dev/null 2>&1 && xdg-icon-resource uninstall --novendor --size 256 goto >/dev/null 2>&1 || true

removed=0
for f in "$BIN" "$ICON" "$APP_DESKTOP" "$AUTOSTART"; do
  if [ -e "$f" ]; then rm -f "$f" && echo ">> removed $f" && removed=1; fi
done
for d in "$DATA" "$CONFIG"; do
  if [ -d "$d" ]; then rm -rf "$d" && echo ">> removed $d" && removed=1; fi
done

# refresh the desktop/icon caches so the launcher entry disappears
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" >/dev/null 2>&1 || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true

echo ""
if [ "$removed" = 1 ]; then
  echo "goto uninstalled."
else
  echo "nothing to remove (goto was not installed for this user)."
fi
echo "note: if you added the apt/dnf repo, remove it too:"
echo "  Debian/Ubuntu: sudo apt remove goto && sudo rm /etc/apt/sources.list.d/goto.list /usr/share/keyrings/goto.gpg"
echo "  Fedora/RHEL:   sudo dnf remove goto && sudo rm /etc/yum.repos.d/goto.repo"
