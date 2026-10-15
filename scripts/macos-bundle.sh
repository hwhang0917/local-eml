#!/bin/sh
# Wraps a darwin local-eml binary in a Local Eml.app bundle and zips it.
#
# A bare binary downloaded by a browser loses its executable bit and, once
# chmod'ed, double-clicking it opens a Terminal window. A bundle fixes both:
# zip keeps the mode and Finder launches it like any app. Must run on macOS
# (sips, iconutil, codesign, ditto).
#
# usage: macos-bundle.sh <binary> <version> <out.zip>
set -eu

BIN=$1
VERSION=$2
OUT=$3

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# Same artwork the Windows resources use (cmd/local-eml/winres/winres.json).
ICON_SRC="$ROOT/web/public/apple-touch-icon.png"
APP_NAME="Local Eml"
BUNDLE_ID="io.github.hwhang0917.local-eml"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

APP="$WORK/$APP_NAME.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
install -m 0755 "$BIN" "$APP/Contents/MacOS/local-eml"

# .icns from the PNG: iconutil wants an .iconset of fixed sizes. The source is
# 180px, so 512 and 1024 are upscaled; the icon is flat so that's fine.
ICONSET="$WORK/AppIcon.iconset"
mkdir "$ICONSET"
for s in 16 32 128 256 512; do
  sips -z "$s" "$s" "$ICON_SRC" --out "$ICONSET/icon_${s}x${s}.png" >/dev/null
  d=$((s * 2))
  sips -z "$d" "$d" "$ICON_SRC" --out "$ICONSET/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"

# LSUIElement: the UI lives in the browser window, so no Dock icon or menu bar
# of its own; the process exits when the window closes, like on Windows.
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>$APP_NAME</string>
  <key>CFBundleDisplayName</key><string>$APP_NAME</string>
  <key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
  <key>CFBundleExecutable</key><string>local-eml</string>
  <key>CFBundleIconFile</key><string>AppIcon</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.productivity</string>
  <key>LSUIElement</key><true/>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST
plutil -lint "$APP/Contents/Info.plist"

# Ad-hoc signature: not notarized (Gatekeeper still asks once), but it seals
# the plist and icon and gives both architectures the same identity.
codesign --force --sign - --identifier "$BUNDLE_ID" "$APP"
codesign --verify --deep --strict "$APP"

# ditto keeps modes and extended attributes, which plain zip may drop.
ditto -c -k --keepParent "$APP" "$OUT"
ls -lh "$OUT"
