#!/bin/bash
set -e

# VibeCoding Tarball Builder
# Usage: ./scripts/build-tarball.sh <arch> [version]
# Example: ./scripts/build-tarball.sh amd64 v0.0.2

BINARY_NAME="vibecoding"
PACKAGE_NAME="vibecoding"

# Parse arguments
OS="${1:-linux}"
ARCH="${2:-amd64}"
VERSION="${3:-$(git describe --tags --always 2>/dev/null || echo "0.0.1")}"

# Remove leading 'v' if present
VERSION="${VERSION#v}"
VERSION="${VERSION%-dirty}"

BUILD_DIR="dist/tarball"
TARBALL_NAME="${PACKAGE_NAME}-${VERSION}-${OS}-${ARCH}"

echo "Building ${TARBALL_NAME}..."

# Clean previous build
rm -rf "${BUILD_DIR}/${TARBALL_NAME}"
mkdir -p "${BUILD_DIR}/${TARBALL_NAME}"

# Check if binary exists
BINARY_FILE="bin/${BINARY_NAME}-${OS}-${ARCH}"
if [ "${OS}" = "windows" ]; then
    BINARY_FILE="${BINARY_FILE}.exe"
fi

if [ ! -f "${BINARY_FILE}" ]; then
    echo "Error: Binary not found: ${BINARY_FILE}"
    echo "Run 'make build-${OS}' first or 'make build-all'"
    exit 1
fi

# Copy binary
cp "${BINARY_FILE}" "${BUILD_DIR}/${TARBALL_NAME}/${BINARY_NAME}"
if [ "${OS}" = "windows" ]; then
    mv "${BUILD_DIR}/${TARBALL_NAME}/${BINARY_NAME}" "${BUILD_DIR}/${TARBALL_NAME}/${BINARY_NAME}.exe"
fi

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

# Create install script for Unix
if [ "${OS}" != "windows" ]; then
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
fi

# Create tarball
echo "Creating tarball..."
cd "${BUILD_DIR}"
tar -czf "${TARBALL_NAME}.tar.gz" "${TARBALL_NAME}"

# Generate checksums
sha256sum "${TARBALL_NAME}.tar.gz" > "${TARBALL_NAME}.tar.gz.sha256"
cd - > /dev/null

# Cleanup temp directory
rm -rf "${BUILD_DIR}/${TARBALL_NAME}"

echo "  Created: ${BUILD_DIR}/${TARBALL_NAME}.tar.gz"
