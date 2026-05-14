#!/usr/bin/env bash
set -euo pipefail

# Show error location on failure
trap 'error "Installation failed at line $LINENO."' ERR

# VibeCoding Installer
# Downloads and installs the latest release from GitHub
#
# Supports non-root installation to ~/.vibecoding/bin
#
# Repository: https://github.com/fuckvibecoding/vibecoding
# Author:     zhenruyan
# Blog:       https://pkold.com

REPO="fuckvibecoding/vibecoding"
BINARY_NAME="vibecoding"

# User-level install directory (no root required)
USER_INSTALL_DIR="${HOME}/.vibecoding/bin"

# Default install directory: auto-detect based on write permission
# Priority: INSTALL_DIR env > writable /usr/local/bin > ~/.vibecoding/bin
if [ -n "${INSTALL_DIR:-}" ]; then
    : # User explicitly set INSTALL_DIR
elif [ -w "/usr/local/bin" ] || [ -w "/usr/local" ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="$USER_INSTALL_DIR"
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

# Install binary (no sudo for user-level install)
install_binary() {
    local binary_path="$1"
    
    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        info "Creating install directory: ${INSTALL_DIR}"
        mkdir -p "$INSTALL_DIR" || error "Failed to create ${INSTALL_DIR}. Check permissions."
    fi
    
    # Install binary
    mv "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    
    success "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Detect user's shell and config file
detect_shell_config() {
    local shell_name
    shell_name="$(basename "${SHELL:-bash}")"
    
    case "$shell_name" in
        zsh)
            # Prefer .zshenv for PATH (loaded by all zsh modes)
            if [ -f "${HOME}/.zshenv" ]; then
                echo "${HOME}/.zshenv"
            elif [ -f "${HOME}/.zshrc" ]; then
                echo "${HOME}/.zshrc"
            else
                echo "${HOME}/.zshenv"
            fi
            ;;
        bash)
            # .bashrc is most common; .bash_profile for login shells on macOS
            if [ -f "${HOME}/.bashrc" ]; then
                echo "${HOME}/.bashrc"
            elif [ -f "${HOME}/.bash_profile" ]; then
                echo "${HOME}/.bash_profile"
            else
                if [ "$(uname -s)" = "Darwin" ]; then
                    echo "${HOME}/.bash_profile"
                else
                    echo "${HOME}/.bashrc"
                fi
            fi
            ;;
        fish)
            echo "${HOME}/.config/fish/config.fish"
            ;;
        *)
            echo "${HOME}/.profile"
            ;;
    esac
}

# Add INSTALL_DIR to PATH in shell config
add_to_path() {
    local config_file="$1"
    local config_dir
    config_dir="$(dirname "$config_file")"
    
    # Create config directory if needed (e.g. ~/.config/fish/)
    if [ ! -d "$config_dir" ]; then
        mkdir -p "$config_dir"
    fi
    
    # Create config file if it doesn't exist
    if [ ! -f "$config_file" ]; then
        touch "$config_file"
    fi
    
    # Check if already in PATH config
    if grep -q "\.vibecoding/bin" "$config_file" 2>/dev/null; then
        info "PATH already configured in ${config_file}"
        return 0
    fi
    
    local shell_name
    shell_name="$(basename "${SHELL:-bash}")"
    
    local path_line
    case "$shell_name" in
        fish)
            path_line="set -gx PATH ${INSTALL_DIR} \$PATH"
            ;;
        *)
            path_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
            ;;
    esac
    
    echo "" >> "$config_file"
    echo "# VibeCoding" >> "$config_file"
    echo "$path_line" >> "$config_file"
    
    success "Added ${INSTALL_DIR} to PATH in ${config_file}"
}

# Check if installed directory is in PATH
check_path() {
    # If already in PATH, nothing to do
    if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        return 0
    fi
    
    # For user-level install, auto-add to shell config
    if [ "$INSTALL_DIR" = "$USER_INSTALL_DIR" ]; then
        local config_file
        config_file=$(detect_shell_config)
        
        echo ""
        info "Detected shell: $(basename "${SHELL:-bash}")"
        info "Shell config: ${config_file}"
        echo ""
        
        # Ask user (default yes)
        local answer
        read -rp "Add ${INSTALL_DIR} to PATH automatically? [Y/n] " answer
        answer="${answer:-Y}"
        
        if [[ "$answer" =~ ^[Yy]$ ]]; then
            add_to_path "$config_file"
            echo ""
            warn "Run this to apply immediately:"
            echo ""
            local shell_name
            shell_name="$(basename "${SHELL:-bash}")"
            case "$shell_name" in
                fish)
                    echo -e "  ${CYAN}source ${config_file}${NC}"
                    ;;
                *)
                    echo -e "  ${CYAN}source ${config_file}${NC}"
                    ;;
            esac
            echo ""
        else
            warn "${INSTALL_DIR} is not in your PATH"
            echo ""
            echo "Add it manually to your shell configuration:"
            echo ""
            echo -e "  ${CYAN}# bash/zsh:${NC}"
            echo -e "  ${CYAN}export PATH=\"${INSTALL_DIR}:\$PATH\"${NC}"
            echo ""
            echo -e "  ${CYAN}# fish:${NC}"
            echo -e "  ${CYAN}set -gx PATH ${INSTALL_DIR} \$PATH${NC}"
            echo ""
        fi
    else
        # System install dir not in PATH (unusual)
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
    
    # Show install mode
    if [ "$INSTALL_DIR" = "$USER_INSTALL_DIR" ]; then
        info "Install mode: user-level (no root required)"
    else
        info "Install mode: system-level"
    fi
    info "Install directory: ${INSTALL_DIR}"
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
    
    # Install binary (no sudo needed for user-level)
    install_binary "$binary_path"
    
    # Check PATH and offer to configure
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
        echo "  Binary installed to:"
        echo "    ${INSTALL_DIR}/${BINARY_NAME}"
        echo ""
        echo "  To use right now:"
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo "    ${BINARY_NAME} --help"
        echo ""
    fi
}

main "$@"
