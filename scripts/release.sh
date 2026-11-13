#!/bin/sh
# Builds Reinvent.app (universal: Apple Silicon + Intel), verifies it the way
# another Mac would run it, and with --publish, tags and creates the GitHub release.
#
#   scripts/release.sh 1.0.0            build + verify into dist/, publish nothing
#   scripts/release.sh 1.0.0 --publish  ...then tag v1.0.0 and `gh release create`
#
# Signing: Developer ID + notarisation when this Mac has both the certificate
# and the `notarytool` keychain profile; otherwise ad-hoc. Ad-hoc is fine for
# the curl installer (install.sh), which macOS does not quarantine.
set -eu

V=${1:?usage: scripts/release.sh <version> [--publish]}
PUBLISH=${2:-}
APP=Reinvent
BUNDLE_ID=com.fintonlabs.reinvent
REPO=$(sed -n 's/^const githubRepo = "\(.*\)"/\1/p' app/workspace.go)
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
dist="$root/dist"
die() { echo "release: $*" >&2; exit 1; }

# 1. Refuse first, build second.
case "$V" in [0-9]*.[0-9]*.[0-9]*) ;; *) die "version must look like 1.2.3" ;; esac
if [ "$PUBLISH" = "--publish" ]; then
  git rev-parse --git-dir >/dev/null 2>&1 || die "not a git repo"
  [ -z "$(git status --porcelain)" ] || die "working tree is dirty"
  git fetch --tags --quiet origin
  ! git rev-parse -q --verify "refs/tags/v$V" >/dev/null || die "tag v$V already exists"
  gh auth status >/dev/null 2>&1 || die "gh is not logged in"
elif [ -n "$PUBLISH" ]; then
  die "unknown option $PUBLISH"
fi
IDENTITY=$(security find-identity -v -p codesigning 2>/dev/null | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -n 1)
NOTARISE=no
if [ -n "$IDENTITY" ]; then
  if xcrun notarytool history --keychain-profile notarytool >/dev/null 2>&1; then
    NOTARISE=yes
  else
    echo "release: Developer ID found but no working 'notarytool' profile; signing without notarisation" >&2
  fi
fi
echo "==> tests"
(cd app && go vet ./... && go test ./...)

# 2-3. Build universal binary, bundle, sign.
echo "==> build $APP $V"
rm -rf "$dist" && mkdir -p "$dist/stage"
for arch in arm64 amd64; do
  CGO_ENABLED=0 GOOS=darwin GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$V" -o "$dist/reinvent-$arch" ./app
done
app="$dist/stage/$APP.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
lipo -create -output "$app/Contents/MacOS/$APP" "$dist/reinvent-arm64" "$dist/reinvent-amd64"
cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>$APP</string>
  <key>CFBundleDisplayName</key><string>$APP</string>
  <key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
  <key>CFBundleExecutable</key><string>$APP</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$V</string>
  <key>CFBundleVersion</key><string>$V</string>
  <key>LSMinimumSystemVersion</key><string>12.0</string>
  <key>LSUIElement</key><true/>
</dict></plist>
PLIST
if [ -n "$IDENTITY" ]; then
  echo "==> sign: $IDENTITY"
  codesign --force --options runtime --timestamp -s "$IDENTITY" "$app"
else
  echo "==> sign: ad-hoc (no Developer ID on this Mac)"
  codesign --force -s - "$app"
fi
codesign --verify --strict "$app"

# 4. Notarise and staple when possible.
if [ "$NOTARISE" = yes ]; then
  echo "==> notarise"
  ditto -c -k --keepParent "$app" "$dist/notary.zip"
  out=$(xcrun notarytool submit "$dist/notary.zip" --keychain-profile notarytool --wait 2>&1) || true
  echo "$out" | grep -q "status: Accepted" || { echo "$out" >&2; die "notarisation refused"; }
  xcrun stapler staple "$app"
  rm "$dist/notary.zip"
fi

# 5. The release artifact.
zip="$dist/$APP-macos.zip"
ditto -c -k --keepParent "$app" "$zip"

# 7. Verify the artifact, not the intention: unpack a copy, run it from
# outside the checkout with an empty home, so nothing here can leak in.
echo "==> verify"
check=$(mktemp -d); mkdir -p "$check/home" "$check/cwd"  # outside the checkout
ditto -x -k "$zip" "$check"
bin="$check/$APP.app/Contents/MacOS/$APP"
lipo -archs "$bin" | grep -q "x86_64 arm64\|arm64 x86_64" || die "binary is not universal"
codesign --verify --strict "$check/$APP.app" || die "signature broken in the zip"
if strings -a "$bin" | grep -q "$HOME"; then die "binary contains a path from this Mac ($HOME)"; fi
got=$(cd "$check/cwd" && HOME="$check/home" "$bin" version)
[ "$got" = "reinvent $V" ] || die "version check failed: $got"
(cd "$check/cwd" && HOME="$check/home" PATH=/usr/bin:/bin "$bin" run -dry-run -no-wait >/dev/null) || die "dry run from a clean home failed"
[ -f "$check/home/Library/Application Support/Reinvent/terraform/main.tf" ] || die "workspace was not unpacked"
rm -rf "$check" "$dist/stage" "$dist"/reinvent-*
echo "==> ok: $zip ($(du -h "$zip" | cut -f1 | tr -d ' '))"

[ "$PUBLISH" = "--publish" ] || { echo "Not published (pass --publish to tag v$V and create the GitHub release)."; exit 0; }

# 8. Only now: tag and release.
notes="$dist/notes.md"
cat > "$notes" <<NOTES
**Install (Mac):** open Terminal and paste

\`\`\`
curl -fsSL https://raw.githubusercontent.com/$REPO/main/install.sh | sh
\`\`\`

All you need is a PagerDuty API key.
NOTES
git tag -a "v$V" -m "Reinvent $V"
git push origin "v$V"
gh release create "v$V" "$zip" -R "$REPO" --title "Reinvent $V" --notes-file "$notes"
