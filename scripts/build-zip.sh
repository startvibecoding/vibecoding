#!/bin/bash
set -e

# VibeCoding Windows Zip Builder
# Usage: ./scripts/build-zip.sh <arch> [version]
# Example: ./scripts/build-zip.sh amd64 v0.0.2

BINARY_NAME="vibecoding"
PACKAGE_NAME="vibecoding"

# Parse arguments
ARCH="${1:-amd64}"
VERSION="${2:-$(git describe --tags --always 2>/dev/null || echo "0.0.1")}"

# Remove leading 'v' if present
VERSION="${VERSION#v}"
VERSION="${VERSION%-dirty}"

BUILD_DIR="dist/zip"
ZIP_NAME="${PACKAGE_NAME}-${VERSION}-windows-${ARCH}"

echo "Building ${ZIP_NAME}..."

# Clean previous build
rm -rf "${BUILD_DIR}/${ZIP_NAME}"
mkdir -p "${BUILD_DIR}/${ZIP_NAME}"

# Check if binary exists
BINARY_FILE="bin/${BINARY_NAME}-windows-${ARCH}.exe"
if [ ! -f "${BINARY_FILE}" ]; then
    echo "Error: Binary not found: ${BINARY_FILE}"
    echo "Run 'make build-windows' first or 'make build-all'"
    exit 1
fi

# Copy binary
cp "${BINARY_FILE}" "${BUILD_DIR}/${ZIP_NAME}/${BINARY_NAME}.exe"

# Copy documentation
echo "Copying documentation..."
if [ -d "docs" ]; then
    cp -r docs "${BUILD_DIR}/${ZIP_NAME}/"
fi

# Copy README
if [ -f "README.md" ]; then
    cp README.md "${BUILD_DIR}/${ZIP_NAME}/"
fi

# Copy LICENSE if exists
if [ -f "LICENSE" ]; then
    cp LICENSE "${BUILD_DIR}/${ZIP_NAME}/"
fi

# Copy install scripts
if [ -f "install.ps1" ]; then
    cp install.ps1 "${BUILD_DIR}/${ZIP_NAME}/"
fi

# Create install batch file
cat > "${BUILD_DIR}/${ZIP_NAME}/install.bat" << 'EOF'
@echo off
setlocal

set BINARY_NAME=vibecoding.exe
set INSTALL_DIR=%LOCALAPPDATA%\vibecoding

echo Installing %BINARY_NAME% to %INSTALL_DIR%...

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
copy /Y "%BINARY_NAME%" "%INSTALL_DIR%\"

:: Add to PATH if not already present
echo %PATH% | find /I "%INSTALL_DIR%" > nul
if errorlevel 1 (
    echo Adding %INSTALL_DIR% to PATH...
    setx PATH "%PATH%;%INSTALL_DIR%"
)

echo.
echo ==========================================
echo Installation complete!
echo.
echo Please restart your terminal to use vibecoding.
echo.
echo Run 'vibecoding --help' to get started.
echo ==========================================
pause
EOF

# Create uninstall batch file
cat > "${BUILD_DIR}/${ZIP_NAME}/uninstall.bat" << 'EOF'
@echo off
setlocal

set BINARY_NAME=vibecoding.exe
set INSTALL_DIR=%LOCALAPPDATA%\vibecoding

if exist "%INSTALL_DIR%\%BINARY_NAME%" (
    echo Removing %INSTALL_DIR%\%BINARY_NAME%...
    del /F "%INSTALL_DIR%\%BINARY_NAME%"
    rmdir /Q "%INSTALL_DIR%" 2>nul
    echo Done.
) else (
    echo Binary not found in %INSTALL_DIR%
    echo You may need to remove it manually.
)

pause
EOF

# Create zip
echo "Creating zip..."
cd "${BUILD_DIR}"
if command -v zip &> /dev/null; then
    zip -r "${ZIP_NAME}.zip" "${ZIP_NAME}"
elif command -v 7z &> /dev/null; then
    7z a "${ZIP_NAME}.zip" "${ZIP_NAME}"
else
    echo "Error: Neither zip nor 7z found. Please install one of them."
    exit 1
fi

# Generate checksums
sha256sum "${ZIP_NAME}.zip" > "${ZIP_NAME}.zip.sha256"
cd - > /dev/null

# Cleanup temp directory
rm -rf "${BUILD_DIR}/${ZIP_NAME}"

echo "  Created: ${BUILD_DIR}/${ZIP_NAME}.zip"
