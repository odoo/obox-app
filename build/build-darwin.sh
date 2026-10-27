#!/bin/bash
set -e

# ── Usage ─────────────────────────────────────────────────────────────────────
usage() {
  echo "Usage: $0 [--apple-id EMAIL] [--team-id TEAM_ID] [--app-password APP_PASSWORD] [--sign-identity IDENTITY]"
  echo ""
  echo "  --apple-id      Apple ID email (e.g. your@apple.com)"
  echo "  --team-id       Apple Team ID (e.g. XXXXXXXXXX)"
  echo "  --app-password  App-specific password from appleid.apple.com"
  echo "  --sign-identity Signing identity from keychain"
  echo ""
  echo "  If not provided, ad-hoc signing is used (no notarization)"
  exit 1
}

# ── Config ────────────────────────────────────────────────────────────────────
APP_NAME="Obox App"
BINARY_NAME="obox-app"
DMG_NAME="obox-app-osx-arm64"

# ── Parse args ────────────────────────────────────────────────────────────────
APPLE_ID=""
TEAM_ID=""
APP_PASSWORD=""
SIGN_IDENTITY="-"

while [[ "$#" -gt 0 ]]; do
  case $1 in
    --apple-id) APPLE_ID="$2"; shift ;;
    --team-id) TEAM_ID="$2"; shift ;;
    --app-password) APP_PASSWORD="$2"; shift ;;
    --sign-identity) SIGN_IDENTITY="$2"; shift ;;
    --help) usage ;;
    *) echo "Unknown parameter: $1"; usage ;;
  esac
  shift
done

# ── Derived ───────────────────────────────────────────────────────────────────
APP_BUNDLE="build/bin/${APP_NAME}.app"
BINARY="${APP_BUNDLE}/Contents/MacOS/${BINARY_NAME}"
FRAMEWORKS="${APP_BUNDLE}/Contents/Frameworks"
LIBUSB_SRC=$(brew --prefix libusb)/lib/libusb-1.0.0.dylib
OUTPUT_DMG="build/bin/${DMG_NAME}.dmg"
ENTITLEMENTS="build/darwin/entitlements.plist"

# ── Entitlements ──────────────────────────────────────────────────────────────
echo "▶ Creating entitlements..."
mkdir -p build/darwin
cat > "${ENTITLEMENTS}" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.security.cs.allow-unsigned-executable-memory</key>
    <true/>
    <key>com.apple.security.cs.disable-library-validation</key>
    <true/>
</dict>
</plist>
EOF

# ── Build ─────────────────────────────────────────────────────────────────────
echo "▶ Building..."
wails build -clean

# ── Bundle libusb ─────────────────────────────────────────────────────────────
echo "▶ Bundling libusb..."
mkdir -p "${FRAMEWORKS}"
cp "${LIBUSB_SRC}" "${FRAMEWORKS}/"
install_name_tool -change \
  "${LIBUSB_SRC}" \
  "@executable_path/../Frameworks/libusb-1.0.0.dylib" \
  "${BINARY}"

# ── Sign ──────────────────────────────────────────────────────────────────────
echo "▶ Signing libusb..."
codesign --force --sign "${SIGN_IDENTITY}" \
  --options runtime \
  "${FRAMEWORKS}/libusb-1.0.0.dylib"

echo "▶ Signing app..."
codesign --force --deep --sign "${SIGN_IDENTITY}" \
  --options runtime \
  --entitlements "${ENTITLEMENTS}" \
  "${APP_BUNDLE}"

# ── DMG ───────────────────────────────────────────────────────────────────────
echo "▶ Creating DMG..."
DMG_STAGING="build/bin/dmg_staging"
rm -rf "${DMG_STAGING}"
mkdir -p "${DMG_STAGING}"
cp -r "${APP_BUNDLE}" "${DMG_STAGING}/"
ln -s /Applications "${DMG_STAGING}/Applications"

hdiutil create \
  -volname "${APP_NAME}" \
  -srcfolder "${DMG_STAGING}" \
  -ov -format UDZO \
  "${OUTPUT_DMG}"

rm -rf "${DMG_STAGING}"

# ── Notarize (only if all credentials provided) ───────────────────────────────
if [[ -n "${APPLE_ID}" && -n "${TEAM_ID}" && -n "${APP_PASSWORD}" && "${SIGN_IDENTITY}" != "-" ]]; then
  echo "▶ Notarizing..."
  xcrun notarytool submit "${OUTPUT_DMG}" \
    --apple-id "${APPLE_ID}" \
    --team-id "${TEAM_ID}" \
    --password "${APP_PASSWORD}" \
    --wait

  echo "▶ Stapling (with retry)..."
  for i in 1 2 3 4 5; do
    echo "  Attempt ${i}/5..."
    xcrun stapler staple "${OUTPUT_DMG}" && break
    echo "  Waiting 30s for Apple CDN..."
    sleep 30
  done
else
  echo "⚠ Skipping notarization (missing credentials or ad-hoc signing)"
fi

echo "✅ Done! → ${OUTPUT_DMG}"