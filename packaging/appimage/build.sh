#!/usr/bin/env bash
# Build goto-x86_64.AppImage (portable, no root).
# Requires: appimagetool on PATH (https://github.com/AppImage/AppImageKit).
# Run from the repo root: bash packaging/appimage/build.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

echo ">> building lib + binary (with voice)"
make lib
make build-voice

APPDIR="$ROOT/build/goto.AppDir"
rm -rf "$APPDIR"
install -d "$APPDIR/usr/bin"
install -d "$APPDIR/usr/share/applications"
install -d "$APPDIR/usr/share/icons/hicolor/256x256/apps"

install -m0755 goto "$APPDIR/usr/bin/goto"
install -m0644 packaging/goto.desktop "$APPDIR/usr/share/applications/goto.desktop"
install -m0644 packaging/goto.desktop "$APPDIR/goto.desktop"
install -m0644 packaging/icons/goto.png "$APPDIR/usr/share/icons/hicolor/256x256/apps/goto.png"
install -m0644 packaging/icons/goto.png "$APPDIR/goto.png"

cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/goto" "$@"
EOF
chmod +x "$APPDIR/AppRun"

echo ">> packaging AppImage"
ARCH=x86_64 appimagetool "$APPDIR" "$ROOT/build/goto-x86_64.AppImage"
echo ">> done: build/goto-x86_64.AppImage"
