#!/bin/bash
set -e

# VibeCoding Debian Package Builder
# Usage: ./scripts/build-deb.sh [version]

BINARY_NAME="vibecoding"
PACKAGE_NAME="vibecoding"
MAINTAINER="VibeCoding Team <team@vibecoding.dev>"
DESCRIPTION="AI-powered terminal coding assistant"
HOMEPAGE="https://github.com/fuckvibecoding/vibecoding"

# Get version from argument or git
VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.0.1")}"
# Remove leading 'v' if present
VERSION="${VERSION#v}"

ARCH="amd64"
BUILD_DIR="dist/deb"
PACKAGE_DIR="${BUILD_DIR}/${PACKAGE_NAME}_${VERSION}_${ARCH}"

echo "Building ${PACKAGE_NAME} ${VERSION} for ${ARCH}..."

# Clean previous build
rm -rf "${BUILD_DIR}"
mkdir -p "${PACKAGE_DIR}/DEBIAN"
mkdir -p "${PACKAGE_DIR}/usr/bin"
mkdir -p "${PACKAGE_DIR}/usr/share/doc/${PACKAGE_NAME}"
mkdir -p "${PACKAGE_DIR}/usr/share/licenses/${PACKAGE_NAME}"

# Build binary
echo "Building binary..."
make build

# Copy binary
cp "bin/${BINARY_NAME}" "${PACKAGE_DIR}/usr/bin/"

# Create control file
cat > "${PACKAGE_DIR}/DEBIAN/control" << EOF
Package: ${PACKAGE_NAME}
Version: ${VERSION}
Section: devel
Priority: optional
Architecture: ${ARCH}
Depends: libc6
Maintainer: ${MAINTAINER}
Homepage: ${HOMEPAGE}
Description: ${DESCRIPTION}
 VibeCoding is a terminal-based AI coding assistant that supports
 multiple LLM providers, sandboxed execution, and a rich TUI.
 It helps developers write, edit, and understand code using AI.
EOF

# Create copyright file
cat > "${PACKAGE_DIR}/usr/share/licenses/${PACKAGE_NAME}/copyright" << EOF
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: ${PACKAGE_NAME}
Upstream-Contact: ${MAINTAINER}
Source: ${HOMEPAGE}

Files: *
Copyright: 2024 VibeCoding Team
License: MIT
 Permission is hereby granted, free of charge, to any person obtaining a
 copy of this software and associated documentation files (the "Software"),
 to deal in the Software without restriction, including without limitation
 the rights to use, copy, modify, merge, publish, distribute, sublicense,
 and/or sell copies of the Software, and to permit persons to whom the
 Software is furnished to do so, subject to the following conditions:
 .
 The above copyright notice and this permission notice shall be included
 in all copies or substantial portions of the Software.
 .
 THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
 THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
 DEALINGS IN THE SOFTWARE.
EOF

# Create changelog
cat > "${PACKAGE_DIR}/usr/share/doc/${PACKAGE_NAME}/changelog.Debian" << EOF
${PACKAGE_NAME} (${VERSION}) unstable; urgency=low

  * Release ${VERSION}

 -- ${MAINTAINER}  $(date -R)
EOF

# Compress changelog
gzip -9 -n "${PACKAGE_DIR}/usr/share/doc/${PACKAGE_NAME}/changelog.Debian"

# Create postinst script (optional)
cat > "${PACKAGE_DIR}/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e

echo "VibeCoding installed successfully!"
echo "Run 'vibecoding' to get started."
EOF
chmod 755 "${PACKAGE_DIR}/DEBIAN/postinst"

# Create postrm script
cat > "${PACKAGE_DIR}/DEBIAN/postrm" << 'EOF'
#!/bin/bash
set -e

# Clean up any temporary files if needed
true
EOF
chmod 755 "${PACKAGE_DIR}/DEBIAN/postrm"

# Build the package
echo "Building .deb package..."
dpkg-deb --root-owner-group --build "${PACKAGE_DIR}"

# Generate checksums
cd "${BUILD_DIR}"
DEB_FILE="${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
sha256sum "${DEB_FILE}" > "${DEB_FILE}.sha256"
md5sum "${DEB_FILE}" > "${DEB_FILE}.md5"
cd - > /dev/null

echo ""
echo "=========================================="
echo "Build complete!"
echo "Package: ${BUILD_DIR}/${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
echo ""
echo "Install with:"
echo "  sudo dpkg -i ${BUILD_DIR}/${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
echo ""
echo "Check with:"
echo "  dpkg -I ${BUILD_DIR}/${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
echo "=========================================="
