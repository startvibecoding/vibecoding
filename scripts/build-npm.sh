#!/bin/bash

# Build npm package with embedded binaries for all platforms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
NPM_DIR="$PROJECT_ROOT/npm"
BIN_DIR="$NPM_DIR/bin"
BUILD_DIR="$PROJECT_ROOT/bin"

ensure_wrapper() {
  mkdir -p "$NPM_DIR/bin"
  cp "$SCRIPT_DIR/npm-installer-wrapper.js" "$NPM_DIR/bin/vibecoding"
  chmod +x "$NPM_DIR/bin/vibecoding"
}

# Clean and create bin directory
rm -rf "$BIN_DIR"
mkdir -p "$BIN_DIR"

# Check if binaries exist
ensure_wrapper

if [ ! -d "$BUILD_DIR" ]; then
  echo "Error: Build directory not found. Run 'make build-all' first."
  exit 1
fi

echo "Copying binaries to npm package..."

# Copy all platform binaries
COPIED=0
for binary in "$BUILD_DIR"/vibecoding-*; do
  if [ -f "$binary" ]; then
    filename=$(basename "$binary")
    cp "$binary" "$BIN_DIR/$filename"
    echo "  Copied: $filename"
    COPIED=$((COPIED + 1))
  fi
done

if [ $COPIED -eq 0 ]; then
  echo "Error: No binaries found in $BUILD_DIR"
  echo "Run 'make build-all' first to build all platform binaries."
  exit 1
fi

echo ""
echo "Copied $COPIED binaries to $BIN_DIR"
echo ""

# List binaries with sizes
echo "Package contents:"
ls -lh "$BIN_DIR"/

# Calculate total size
TOTAL_SIZE=$(du -sh "$BIN_DIR" | cut -f1)
echo ""
echo "Total binary size: $TOTAL_SIZE"
