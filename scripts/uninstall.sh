#!/usr/bin/env bash
# Remove the goto install done by install.sh (binary, icon, desktop, autostart).
set -euo pipefail

rm -fv "$HOME/.local/bin/goto" \
       "$HOME/.local/share/icons/hicolor/256x256/apps/goto.png" \
       "$HOME/.local/share/applications/goto.desktop" \
       "$HOME/.config/autostart/goto.desktop" 2>/dev/null || true

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" >/dev/null 2>&1 || true
echo "goto removed. (config and models in ~/.config/goto and ~/.local/share/goto were kept)"
