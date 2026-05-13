#!/usr/bin/env bash
set -euo pipefail

# Show error location on failure
trap 'error "Installation failed at line $LINENO. Set INSTALL_DIR to a writable path or run with sudo."' ERR

# VibeCoding Installer
# Downloads and installs the latest release from GitHub
#
# Repository: https://github.com/fuckvibecoding/vibecoding
# Author:     zhenruyan
# Blog:       https://pkold.com

REPO="fuckvibecoding/vibecoding"
BINARY_NAME="vibecoding"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)          error "Unsupported OS: $(uname -s)" ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)   arch="arm64" ;;
        armv7l|armhf)    arch="arm" ;;
        *)               error "Unsupported architecture: $(uname -m)" ;;
    esac

    echo "${os}/${arch}"
}

# Get latest release version from GitHub
get_latest_version() {
    local version
    version=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
        error "Failed to fetch latest version from GitHub"
    fi
    
    echo "$version"
}

# Download file
download() {
    local url="$1"
    local dest="$2"
    
    info "Downloading: ${url}"
    
    if command -v curl &> /dev/null; then
        curl -sL -o "$dest" "$url"
    elif command -v wget &> /dev/null; then
        wget -qO "$dest" "$url"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Verify checksum
verify_checksum() {
    local file="$1"
    local checksum_file="$2"
    
    if [ ! -f "$checksum_file" ]; then
        warn "Checksum file not found, skipping verification"
        return 0
    fi
    
    local expected
    expected=$(grep "$(basename "$file")" "$checksum_file" | awk '{print $1}' || true)
    
    if [ -z "$expected" ]; then
        warn "Checksum not found for $(basename "$file")"
        return 0
    fi
    
    local actual
    if command -v sha256sum &> /dev/null; then
        actual=$(sha256sum "$file" | awk '{print $1}')
    elif command -v shasum &> /dev/null; then
        actual=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        warn "No sha256sum or shasum found, skipping verification"
        return 0
    fi
    
    if [ "$actual" != "$expected" ]; then
        error "Checksum mismatch: expected ${expected}, got ${actual}"
    fi
    
    success "Checksum verified"
}

# Install binary
install_binary() {
    local binary_path="$1"
    
    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        info "Creating install directory: ${INSTALL_DIR}"
        if [ -w "$(dirname "$INSTALL_DIR")" ]; then
            mkdir -p "$INSTALL_DIR"
        else
            sudo mkdir -p "$INSTALL_DIR" || error "Failed to create ${INSTALL_DIR}. Run with sudo or set INSTALL_DIR to a writable path."
        fi
    fi
    
    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        info "Requesting sudo privileges to install to ${INSTALL_DIR}"
        sudo mv "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}" || error "Failed to move binary. Run with sudo or set INSTALL_DIR to a writable path."
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi
    
    success "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Check if installed directory is in PATH
check_path() {
    if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
        warn "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "Add it to your shell configuration:"
        echo ""
        echo "  For bash/zsh:"
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
        echo "  For fish:"
        echo "    set -gx PATH ${INSTALL_DIR} \$PATH"
        echo ""
    fi
}

# Main installation
main() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║                   VibeCoding Installer                        ║"
    echo "║               https://github.com/fuckvibecoding/vibecoding    ║"
    echo "║                  Author: zhenruyan | pkold.com                ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo ""
    
    # Detect platform
    local platform
    platform=$(detect_platform)
    local os="${platform%/*}"
    local arch="${platform#*/}"
    info "Detected platform: ${os}/${arch}"
    
    # Get latest version
    local version
    version=$(get_latest_version)
    info "Latest version: ${version}"
    
    # Construct download URL
    local archive_name
    local version_num="${version#v}"
    if [ "$os" = "windows" ]; then
        archive_name="${BINARY_NAME}-${version_num}-${os}-${arch}.zip"
    else
        archive_name="${BINARY_NAME}-${version_num}-${os}-${arch}.tar.gz"
    fi
    
    local download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
    local checksum_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
    
    # Create temp directory
    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap "rm -rf ${tmp_dir}" EXIT
    
    # Download archive
    local archive_path="${tmp_dir}/${archive_name}"
    download "$download_url" "$archive_path"
    
    # Download and verify checksum
    local checksum_path="${tmp_dir}/checksums.txt"
    download "$checksum_url" "$checksum_path" 2>/dev/null || true
    verify_checksum "$archive_path" "$checksum_path"
    
    # Extract archive
    info "Extracting archive..."
    local binary_path="${tmp_dir}/${BINARY_NAME}"
    
    if [ "$os" = "windows" ]; then
        if command -v unzip &> /dev/null; then
            unzip -q "$archive_path" -d "$tmp_dir"
        else
            error "unzip not found. Please install unzip."
        fi
        binary_path="${tmp_dir}/${BINARY_NAME}.exe"
    else
        tar -xzf "$archive_path" -C "$tmp_dir"
    fi
    
    if [ ! -f "$binary_path" ]; then
        error "Binary not found in archive"
    fi
    
    # Install binary
    install_binary "$binary_path"
    
    # Check PATH
    check_path
    
    # Verify installation
    echo ""
    if command -v "$BINARY_NAME" &> /dev/null; then
        local installed_version
        installed_version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        success "Installation complete!"
        echo ""
        echo "  Version: ${installed_version}"
        echo ""
        echo "  Get started:"
        echo "    ${BINARY_NAME} --help"
        echo ""
    else
        success "Installation complete!"
        echo ""
        echo "  Restart your shell or run:"
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
        echo "  Then verify with:"
        echo "    ${BINARY_NAME} --help"
        echo ""
    fi
}

main "$@"
