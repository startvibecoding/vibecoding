#!/bin/bash

# Build platform-specific npm packages with optionalDependencies architecture.
# Each platform gets its own package containing only its binary.
# The main package (vibecoding-installer) declares all platforms as optionalDependencies.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
NPM_DIR="$PROJECT_ROOT/npm"
BUILD_DIR="$PROJECT_ROOT/bin"
PACKAGES_DIR="$NPM_DIR/packages"

# Clean packages directory
rm -rf "$PACKAGES_DIR"

# Check if binaries exist
if [ ! -d "$BUILD_DIR" ]; then
  echo "Error: Build directory not found. Run 'make build-all' first."
  exit 1
fi

# Read version from main package.json
VERSION=$(node -e "console.log(require('$NPM_DIR/package.json').version)")

# Platform definitions: npm-cpu -> binary suffix, npm-os -> binary prefix
declare -A PLATFORMS=(
  ["linux-x64"]="vibecoding-linux-amd64"
  ["linux-arm64"]="vibecoding-linux-arm64"
  ["linux-musl-x64"]="vibecoding-linux-musl-amd64"
  ["darwin-x64"]="vibecoding-darwin-amd64"
  ["darwin-arm64"]="vibecoding-darwin-arm64"
  ["win32-x64"]="vibecoding-windows-amd64.exe"
  ["win32-arm64"]="vibecoding-windows-arm64.exe"
)

declare -A OS_MAP=(
  ["linux-x64"]="linux"
  ["linux-arm64"]="linux"
  ["linux-musl-x64"]="linux"
  ["darwin-x64"]="darwin"
  ["darwin-arm64"]="darwin"
  ["win32-x64"]="win32"
  ["win32-arm64"]="win32"
)

declare -A CPU_MAP=(
  ["linux-x64"]="x64"
  ["linux-arm64"]="arm64"
  ["linux-musl-x64"]="x64"
  ["darwin-x64"]="x64"
  ["darwin-arm64"]="arm64"
  ["win32-x64"]="x64"
  ["win32-arm64"]="arm64"
)

BUILT=0
for PLATFORM_KEY in "${!PLATFORMS[@]}"; do
  BINARY_NAME="${PLATFORMS[$PLATFORM_KEY]}"
  OS="${OS_MAP[$PLATFORM_KEY]}"
  CPU="${CPU_MAP[$PLATFORM_KEY]}"
  PKG_NAME="vibecoding-installer-${PLATFORM_KEY}"
  PKG_DIR="$PACKAGES_DIR/$PKG_NAME"

  # Check binary exists
  if [ ! -f "$BUILD_DIR/$BINARY_NAME" ]; then
    echo "Warning: Binary not found: $BUILD_DIR/$BINARY_NAME, skipping $PKG_NAME"
    continue
  fi

  # Create package directory
  mkdir -p "$PKG_DIR/bin"

  # Determine binary name inside package
  if [ "$OS" = "win32" ]; then
    INNER_BINARY="vibecoding.exe"
  else
    INNER_BINARY="vibecoding"
  fi

  # Copy binary
  cp "$BUILD_DIR/$BINARY_NAME" "$PKG_DIR/bin/$INNER_BINARY"
  chmod +x "$PKG_DIR/bin/$INNER_BINARY" 2>/dev/null || true

  # Create package.json
  # For musl packages, set libc="musl" so npm can distinguish from glibc
  # npm >=9.4 supports libc field in package.json for optional dependency resolution
  if echo "$PLATFORM_KEY" | grep -q "musl"; then
    cat > "$PKG_DIR/package.json" <<EOF
{
  "name": "$PKG_NAME",
  "version": "$VERSION",
  "description": "VibeCoding native binary for ${OS}-${CPU} (musl static)",
  "os": ["$OS"],
  "cpu": ["$CPU"],
  "libc": ["musl"],
  "files": ["bin/"],
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/startvibecoding/vibecoding.git",
    "directory": "npm"
  }
}
EOF
  elif echo "$PLATFORM_KEY" | grep -q "^linux-"; then
    cat > "$PKG_DIR/package.json" <<EOF
{
  "name": "$PKG_NAME",
  "version": "$VERSION",
  "description": "VibeCoding native binary for ${OS}-${CPU}",
  "os": ["$OS"],
  "cpu": ["$CPU"],
  "libc": ["glibc"],
  "files": ["bin/"],
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/startvibecoding/vibecoding.git",
    "directory": "npm"
  }
}
EOF
  else
    cat > "$PKG_DIR/package.json" <<EOF
{
  "name": "$PKG_NAME",
  "version": "$VERSION",
  "description": "VibeCoding native binary for ${OS}-${CPU}",
  "os": ["$OS"],
  "cpu": ["$CPU"],
  "files": ["bin/"],
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/startvibecoding/vibecoding.git",
    "directory": "npm"
  }
}
EOF
  fi

  # Calculate size
  SIZE=$(du -sh "$PKG_DIR/bin/$INNER_BINARY" | cut -f1)
  echo "  Built: $PKG_NAME ($OS/$CPU) - $SIZE"
  BUILT=$((BUILT + 1))
done

# Update optionalDependencies versions in main package.json
echo ""
echo "Updating optionalDependencies versions to $VERSION..."
node -e "
const fs = require('fs');
const pkgPath = '$NPM_DIR/package.json';
const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
if (pkg.optionalDependencies) {
  for (const key of Object.keys(pkg.optionalDependencies)) {
    pkg.optionalDependencies[key] = '$VERSION';
  }
}
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + '\n');
console.log('Updated main package.json optionalDependencies');
"

echo ""
echo "Built $BUILT platform packages in $PACKAGES_DIR"
echo ""
echo "Package sizes:"
for d in "$PACKAGES_DIR"/*/; do
  if [ -d "$d" ]; then
    name=$(basename "$d")
    size=$(du -sh "$d" | cut -f1)
    echo "  $name: $size"
  fi
done

# Compare with old single-package approach
echo ""
OLD_SIZE=$(du -sh "$NPM_DIR/bin" 2>/dev/null | cut -f1 || echo "N/A")
echo "Old single-package binary size: $OLD_SIZE"
echo "New per-platform package size:  ~20MB each"
echo "User download: ~20MB (was ~120MB)"
