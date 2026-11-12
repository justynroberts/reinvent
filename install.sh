#!/bin/sh
# Installs Reinvent into your Applications folder and opens it.
#   curl -fsSL https://raw.githubusercontent.com/justynroberts/reinvent/main/install.sh | sh
set -eu
REPO=justynroberts/reinvent
URL="https://github.com/$REPO/releases/latest/download/Reinvent-macos.zip"
DEST="$HOME/Applications"

[ "$(uname -s)" = Darwin ] || { echo "Reinvent runs on macOS only."; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading Reinvent..."
curl -fsSL "$URL" -o "$tmp/Reinvent.zip" || { echo "Download failed. Check your internet connection and try again."; exit 1; }
ditto -x -k "$tmp/Reinvent.zip" "$tmp"
mkdir -p "$DEST"
rm -rf "$DEST/Reinvent.app"
mv "$tmp/Reinvent.app" "$DEST/"
echo "Installed: $DEST/Reinvent.app"
echo "Next time, open it from Spotlight (Cmd+Space, type Reinvent)."
open "$DEST/Reinvent.app"
