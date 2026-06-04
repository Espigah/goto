#!/usr/bin/env bash
# Assembles the GitHub Pages content into ./public:
#   - the landing page (site/*)
#   - a signed APT repo (public/apt) and YUM repo (public/yum), built from the
#     .deb/.rpm assets of ALL GitHub releases, so the repos always reflect
#     every published version.
#
# Signing is gated on the GPG_PRIVATE_KEY secret. Without it, only the landing
# is published (no package repo). Env: GPG_PRIVATE_KEY, GPG_PASSPHRASE (opt),
# GH_TOKEN, REPO (owner/name).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PUB="$ROOT/public"
rm -rf "$PUB"
mkdir -p "$PUB"
cp -r site/. "$PUB"/

if [ -z "${GPG_PRIVATE_KEY:-}" ]; then
  echo ">> no GPG_PRIVATE_KEY secret: publishing the landing only (no package repo)."
  exit 0
fi

echo ">> importing signing key"
echo "$GPG_PRIVATE_KEY" | gpg --batch --import
KEYID="$(gpg --list-secret-keys --with-colons | awk -F: '/^sec/{print $5; exit}')"
gpgsign() { gpg --batch --yes --pinentry-mode loopback --passphrase "${GPG_PASSPHRASE:-}" --default-key "$KEYID" "$@"; }

mkdir -p "$PUB/apt" "$PUB/yum"

echo ">> downloading release assets (.deb/.rpm)"
api="https://api.github.com/repos/${REPO}/releases?per_page=100"
urls="$(curl -fsSL -H "Authorization: Bearer ${GH_TOKEN}" "$api" | jq -r '.[].assets[].browser_download_url')"
for u in $urls; do
  case "$u" in
    *.deb) (cd "$PUB/apt" && curl -fsSL -O "$u") ;;
    *.rpm) (cd "$PUB/yum" && curl -fsSL -O "$u") ;;
  esac
done

echo ">> building APT repo"
( cd "$PUB/apt"
  apt-ftparchive packages . > Packages
  gzip -kf Packages
  apt-ftparchive release . > Release
  gpgsign -abs -o Release.gpg Release
  gpgsign --clearsign -o InRelease Release
)
gpg --armor --export "$KEYID" > "$PUB/apt/key.gpg"

echo ">> building YUM repo"
createrepo_c "$PUB/yum"
gpgsign --detach-sign --armor "$PUB/yum/repodata/repomd.xml"
gpg --armor --export "$KEYID" > "$PUB/yum/key.gpg"

echo ">> done. public/ has landing + apt + yum"
