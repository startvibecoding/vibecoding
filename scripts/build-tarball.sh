#!/bin/bash
set -e

# VibeCoding Tarball Builder
# Usage: ./scripts/build-tarball.sh [version]

BINARY_NAME="vibecoding"
PACKAGE_NAME="vibecoding"

# Get version from argument or git
VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.0.1")}"
# Remove leading 'v' if present
VERSION="${VERSION#v}"

BUILD_DIR="dist/tarball"
TARBALL_NAME="${PACKAGE_NAME}-${VERSION}"

echo "Building ${PACKAGE_NAME} ${VERSION} tarball..."

# Clean previous build
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}/${TARBALL_NAME}"

# Build binary
echo "Building binary..."
make build

# Copy binary
cp "bin/${BINARY_NAME}" "${BUILD_DIR}/${TARBALL_NAME}/"

# Copy documentation
echo "Copying documentation..."
if [ -d "docs" ]; then
    cp -r docs "${BUILD_DIR}/${TARBALL_NAME}/"
fi

# Copy README
if [ -f "README.md" ]; then
    cp README.md "${BUILD_DIR}/${TARBALL_NAME}/"
fi

# Copy LICENSE if exists
if [ -f "LICENSE" ]; then
    cp LICENSE "${BUILD_DIR}/${TARBALL_NAME}/"
fi

# Create install script
cat > "${BUILD_DIR}/${TARBALL_NAME}/install.sh" << 'EOF'
#!/bin/bash
set -e

BINARY_NAME="vibecoding"
INSTALL_DIR="/usr/local/bin"

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Note: Installing to user-local directory"
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

echo "Installing ${BINARY_NAME} to ${INSTALL_DIR}..."
cp "${BINARY_NAME}" "${INSTALL_DIR}/"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo ""
echo "=========================================="
echo "Installation complete!"
echo ""
echo "Make sure ${INSTALL_DIR} is in your PATH"
echo ""
echo "Add to your shell rc file if needed:"
echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
echo ""
echo "Run 'vibecoding' to get started."
echo "=========================================="
EOF
chmod +x "${BUILD_DIR}/${TARBALL_NAME}/install.sh"

# Create uninstall script
cat > "${BUILD_DIR}/${TARBALL_NAME}/uninstall.sh" << 'EOF'
#!/bin/bash
set -e

BINARY_NAME="vibecoding"

# Check common install locations
LOCATIONS=("/usr/local/bin/${BINARY_NAME}" "$HOME/.local/bin/${BINARY_NAME}")

for loc in "${LOCATIONS[@]}"; do
    if [ -f "$loc" ]; then
        echo "Removing ${loc}..."
        rm -f "$loc"
        echo "Done."
        exit 0
    fi
done

echo "Binary not found in common locations."
echo "You may need to remove it manually."
EOF
chmod +x "${BUILD_DIR}/${TARBALL_NAME}/uninstall.sh"

# Create tarball
echo "Creating tarball..."
cd "${BUILD_DIR}"
tar -czf "${TARBALL_NAME}.tar.gz" "${TARBALL_NAME}"

# Generate checksums
sha256sum "${TARBALL_NAME}.tar.gz" > "${TARBALL_NAME}.tar.gz.sha256"
md5sum "${TARBALL_NAME}.tar.gz" > "${TARBALL_NAME}.tar.gz.md5"
cd - > /dev/null

echo ""
echo "=========================================="
echo "Build complete!"
echo "Tarball: ${BUILD_DIR}/${TARBALL_NAME}.tar.gz"
echo ""
echo "Extract with:"
echo "  tar -xzf ${BUILD_DIR}/${TARBALL_NAME}.tar.gz"
echo ""
echo "Install with:"
echo "  cd ${TARBALL_NAME} && ./install.sh"
echo "=========================================="
