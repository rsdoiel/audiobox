#!/bin/bash
set -euo pipefail

# --- Check if running on macOS ---
if [[ "$(uname)" != "Darwin" ]]; then
  echo "Error: This script must be run on macOS."
  exit 1
fi

# --- Extract App Name and Version from codemeta.json ---
CODEMETA_FILE="codemeta.json"
APP_NAME=$(jq -r '.name' "$CODEMETA_FILE" | tr '[:upper:]' '[:lower:]' | tr -d ' ')
APP_VERSION=$(jq -r '.version' "$CODEMETA_FILE")
BINARY_NAME="${APP_NAME}-macos-arm64"
ICNS_NAME="${APP_NAME}.icns"
JPEG_ICON="${APP_NAME}.jpg"
APP_BUNDLE="${APP_NAME}.app"
OUTPUT_DIR="dist"

# --- Configuration ---
GO_CMD="go build -o ${BINARY_NAME} cmd/audiobox/main.go"

# --- Validate codemeta.json ---
if [ ! -f "$CODEMETA_FILE" ]; then
  echo "Error: $CODEMETA_FILE not found."
  exit 1
fi

# --- Validate JPEG Icon ---
if [ ! -f "$JPEG_ICON" ]; then
  echo "Error: Icon file $JPEG_ICON not found."
  exit 1
fi

# --- Build Go Binary ---
echo "Building Go binary for macOS..."
GOOS=darwin GOARCH=arm64 $GO_CMD

# --- Validate Binary ---
if [ ! -f "$BINARY_NAME" ]; then
  echo "Error: Binary $BINARY_NAME not found after build."
  exit 1
fi
echo "Go binary ${BINARY_NAME}"

# --- Create Output Directory ---
mkdir -p "$OUTPUT_DIR"
APP_BUNDLE_PATH="$OUTPUT_DIR/$APP_BUNDLE"

# --- Create .app Bundle Structure ---
echo "Creating .app bundle at $APP_BUNDLE_PATH..."
mkdir -p "$APP_BUNDLE_PATH/Contents/MacOS"
mkdir -p "$APP_BUNDLE_PATH/Contents/Resources"

# --- Convert JPEG to .icns ---
echo "Converting $JPEG_ICON to $ICNS_NAME..."
mkdir -p "${APP_NAME}.iconset"

# Force PNG output for all sizes
sips -s format png -z 16 16     "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_16x16.png"
sips -s format png -z 32 32     "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_16x16@2x.png"
sips -s format png -z 32 32     "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_32x32.png"
sips -s format png -z 64 64     "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_32x32@2x.png"
sips -s format png -z 128 128   "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_128x128.png"
sips -s format png -z 256 256   "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_128x128@2x.png"
sips -s format png -z 256 256   "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_256x256.png"
sips -s format png -z 512 512   "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_256x256@2x.png"
sips -s format png -z 512 512   "$JPEG_ICON" --out "${APP_NAME}.iconset/icon_512x512.png"

# Convert iconset to .icns
if ! iconutil -c icns "${APP_NAME}.iconset" -o "$ICNS_NAME"; then
  echo "Error: Failed to generate ICNS file."
  exit 1
fi
rm -rf "${APP_NAME}.iconset"

# --- Copy Binary and Icon ---
echo "Copying binary and icon to .app bundle..."
cp "${BINARY_NAME}" "${APP_BUNDLE_PATH}/Contents/Resources/$APP_NAME"
cp "${ICNS_NAME}" "${APP_BUNDLE_PATH}/Contents/Resources/$ICNS_NAME"

# --- Generate Info.plist ---
echo "Generating Info.plist..."
cat > "$APP_BUNDLE_PATH/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>$APP_NAME</string>
    <key>CFBundleIconFile</key>
    <string>$ICNS_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>com.github.rsdoiel.$APP_NAME</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>$APP_VERSION</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <false/>
</dict>
</plist>
EOF

# --- Create Executable Shell Script ---
echo "Creating executable shell script..."
cat > "$APP_BUNDLE_PATH/Contents/MacOS/$APP_NAME" <<'SCRIPTEOF'
#!/bin/bash
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
RESOURCES_DIR="$SCRIPT_DIR/../Resources"
exec "$RESOURCES_DIR/$APP_NAME" "$@"
SCRIPTEOF
chmod +x "$APP_BUNDLE_PATH/Contents/MacOS/$APP_NAME"

# --- Clean Up ---
rm "$ICNS_NAME"
rm "$BINARY_NAME"

echo "✅ .app bundle created at: $APP_BUNDLE_PATH"
