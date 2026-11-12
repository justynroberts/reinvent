#!/bin/sh
# The one thing you run for this demo:  ./trigger
# No arguments opens a menu with every step. Rebuilds the app when its code changes.
set -e
root=$(cd "$(dirname "$0")" && pwd)
bin="$root/bin/trigger"
cd "$root"

if [ ! -x "$bin" ] || [ -n "$(find app go.mod assets.go -newer "$bin" \( -name '*.go' -o -name go.mod \) 2>/dev/null | head -n 1)" ]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Go isn't installed. Fix: brew install go   then run ./trigger again." >&2
    exit 1
  fi
  echo "Building the app (first run or code changed)..."
  (cd app && go build -o "$bin" .)
fi
exec "$bin" "$@"
