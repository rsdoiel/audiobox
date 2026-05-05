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
DMG_FINAL="$DMG_NAME.dmg"
VOLUME_NAME="Audiobox"
STAGING_DIR="dmg-staging"  # Temporary directory for staging files

# Files to include in the .dmg
FILES_TO_INCLUDE=(
  "$APP_BUNDLE"
  "INSTALL.md"
  "INSTALL_NOTES_macOS.md"
  "LICENSE"
  "README.md"
)

# --- Validate .app Bundle ---
if [ ! -d "$APP_BUNDLE" ]; then
  echo "Error: .app bundle not found at $APP_BUNDLE. Run the .app bundle creation script first."
  exit 1
fi

# --- Remove existing final .dmg file (if it exists) ---
if [ -f "$DMG_FINAL" ]; then
  echo "Removing existing final disk image: $DMG_FINAL"
  rm -f "$DMG_FINAL"
fi

# --- Step 1: Create staging directory and copy all files ---
echo "Creating staging directory and copying files..."
rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR"

# Copy all specified files to the staging directory
for file in "${FILES_TO_INCLUDE[@]}"; do
  if [ -e "$file" ]; then
    cp -R "$file" "$STAGING_DIR/"
    echo "Copied $(basename "$file") to staging directory"
  else
    echo "Warning: File $file not found. Skipping."
  fi
done

# Create a symbolic link to /Applications in the staging directory
echo "Creating Applications symlink in staging directory..."
ln -sf /Applications "$STAGING_DIR/Applications"

# List contents of the staging directory for verification
echo "Contents of staging directory:"
ls -l "$STAGING_DIR"

# --- Step 2: Create the disk image directly from the staging directory ---
echo "Creating disk image from staging directory..."
hdiutil create -srcfolder "$STAGING_DIR" -volname "$VOLUME_NAME" -ov -format UDZO "$DMG_FINAL"

# --- Step 3: Clean up ---
echo "Cleaning up..."
rm -rf "$STAGING_DIR"

echo "✅ Done! Final disk image: $DMG_FINAL"
