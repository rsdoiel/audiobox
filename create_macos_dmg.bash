#!/bin/bash
set -euo pipefail

# --- Check if running on macOS ---
if [[ "$(uname)" != "Darwin" ]]; then
  echo "Error: This script must be run on macOS."
  exit 1
fi

# --- Configuration ---
APP_NAME="audiobox"
APP_BUNDLE="dist/$APP_NAME.app"  # Path to the .app bundle
DMG_NAME="$APP_NAME-macos"
DMG_TEMP="$DMG_NAME-temp.dmg"
DMG_FINAL="$DMG_NAME.dmg"
BACKGROUND_IMAGE="dmg-background.png"  # Optional
VOLUME_NAME="Audiobox"
VOLUME_PATH="/Volumes/$VOLUME_NAME"

# --- Validate .app Bundle ---
if [ ! -d "$APP_BUNDLE" ]; then
  echo "Error: .app bundle not found at $APP_BUNDLE. Run the .app bundle creation script first."
  exit 1
fi

# --- Step 1: Create a temporary disk image ---
echo "Creating temporary disk image..."
hdiutil create -srcfolder "$APP_BUNDLE" -volname "$VOLUME_NAME" -ov -format UDRW "$DMG_TEMP"

# --- Step 2: Mount the disk image ---
echo "Mounting disk image..."
hdiutil attach "$DMG_TEMP" -noverify -noautoopen

# Wait for the volume to appear
echo "Waiting for volume to mount..."
MAX_ATTEMPTS=10
ATTEMPT=0
while [ ! -d "$VOLUME_PATH" ] && [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
  sleep 1
  ATTEMPT=$((ATTEMPT + 1))
done

if [ ! -d "$VOLUME_PATH" ]; then
  echo "Error: Volume $VOLUME_NAME did not mount after $MAX_ATTEMPTS attempts."
  exit 1
fi

# --- Step 3: Customize the disk image (optional) ---
if [ -f "$BACKGROUND_IMAGE" ]; then
  echo "Customizing disk image appearance..."
  # Copy background image
  mkdir -p "$VOLUME_PATH/.background"
  cp "$BACKGROUND_IMAGE" "$VOLUME_PATH/.background/"

  # Set Finder view options
  echo 'tell application "Finder"
    tell disk "'"$VOLUME_NAME"'"
      set background picture to file ".background:'"$BACKGROUND_IMAGE"'"
      set current view of container window to icon view
      set arrangement of container window to not arranged
      set icon size of container window to 128
      set position of container window to {100, 100}
      set bounds of container window to {400, 300, 800, 500}
    end tell
  end tell' | osascript
fi

# --- Step 4: Unmount the disk image ---
echo "Unmounting disk image..."
if ! hdiutil detach "$VOLUME_PATH"; then
  echo "Warning: Failed to unmount disk image. Trying to force unmount..."
  hdiutil detach "$VOLUME_PATH" -force
fi

# --- Step 5: Convert to final compressed .dmg ---
echo "Creating final compressed disk image..."
hdiutil convert "$DMG_TEMP" -format UDZO -o "dist/$DMG_FINAL"
rm -f "$DMG_TEMP"

echo "✅ Done! Final disk image: $DMG_FINAL"
