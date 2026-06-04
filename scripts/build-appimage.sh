#!/usr/bin/env bash
# Build a portable goto AppImage from an already-built binary.
#
#   scripts/build-appimage.sh [version] [binary]
#
# AppImage is not a sandbox: it just bundles the app + a few shared libraries
# into a single self-mounting file. That is exactly what goto needs, because it
# controls OTHER windows (focus, XTEST keystrokes, global hotkey) which a real
# sandbox would block. We bundle libstdc++/libgomp/libgcc_s so it also runs on
# systems that lack them; the Whisper model still downloads to $HOME on first use.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VER="${1:-dev}"; VER="${VER#v}"             # accept "v0.3.5" or "0.3.5"
ARCH="x86_64"

# Locate the binary: explicit arg, GoReleaser dist/, or a local ./goto.
BIN="${2:-}"
if [ -z "$BIN" ]; then
  BIN="$(find dist -maxdepth 2 -type f -name goto -perm -u+x 2>/dev/null | head -1 || true)"
fi
[ -z "$BIN" ] && [ -x "$ROOT/goto" ] && BIN="$ROOT/goto"
[ -n "$BIN" ] && [ -f "$BIN" ] || { echo "goto binary not found (build it first)"; exit 1; }
echo ">> binary: $BIN"

APPDIR="$ROOT/AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/lib" "$APPDIR/usr/share/icons/hicolor/256x256/apps"

install -Dm755 "$BIN" "$APPDIR/usr/bin/goto"
install -Dm644 "$ROOT/packaging/icons/goto.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/goto.png"
cp "$ROOT/packaging/icons/goto.png" "$APPDIR/goto.png"          # top-level icon (required)

# Bundle the C/C++ runtime libs goto links against. ALSA/Pulse are dlopen'd at
# runtime by miniaudio, so we deliberately do NOT bundle them, the host's copy
# is the right one for that machine's sound server.
for soname in libstdc++.so.6 libgomp.so.1 libgcc_s.so.1; do
  src="$(ldd "$APPDIR/usr/bin/goto" 2>/dev/null | awk -v n="$soname" '$1==n {print $3}')"
  [ -n "$src" ] && [ -f "$src" ] || src="$(find /usr/lib /lib -name "$soname" 2>/dev/null | head -1 || true)"
  if [ -n "$src" ] && [ -f "$src" ]; then
    cp -L "$src" "$APPDIR/usr/lib/$soname"
    echo ">> bundled $soname  <- $src"
  else
    echo ">> note: $soname not found on host, relying on target system"
  fi
done

# Desktop entry (required by appimagetool). StartupWMClass lets the desktop map
# the running window to this icon.
cat > "$APPDIR/goto.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=goto
GenericName=Voice window control
Comment=Focus windows by voice: "goto vscode myproject"
Exec=goto
Icon=goto
Terminal=false
Categories=Utility;Accessibility;
Keywords=voice;window;focus;
StartupWMClass=goto
EOF

# Entry point: prefer the bundled libs, then exec goto with the user's args.
cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
export LD_LIBRARY_PATH="$HERE/usr/lib:${LD_LIBRARY_PATH:-}"
exec "$HERE/usr/bin/goto" "$@"
EOF
chmod +x "$APPDIR/AppRun"

# appimagetool. Use extract-and-run so it works on CI runners without FUSE.
TOOL="$ROOT/appimagetool-x86_64.AppImage"
if [ ! -x "$TOOL" ]; then
  echo ">> fetching appimagetool"
  curl -fsSL "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage" -o "$TOOL"
  chmod +x "$TOOL"
fi

OUT="$ROOT/goto_${VER}_${ARCH}.AppImage"
echo ">> packing $OUT"
ARCH="$ARCH" APPIMAGE_EXTRACT_AND_RUN=1 "$TOOL" "$APPDIR" "$OUT"
echo ">> done: $OUT"
ls -la "$OUT"
