#!/bin/bash
set -e

# androideai-core installer
# Usage: curl -fsSL https://raw.githubusercontent.com/mobiai/androideai-core/main/install.sh | bash

REPO="mobiai/androideai-core"
BINARY="androideai"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case $OS in
        linux)
            case $ARCH in
                x86_64|amd64)
                    PLATFORM="linux_amd64"
                    ;;
                aarch64|arm64)
                    PLATFORM="linux_arm64"
                    ;;
                armv7l|armhf)
                    PLATFORM="linux_armv7"
                    ;;
                *)
                    echo "Unsupported architecture: $ARCH"
                    exit 1
                    ;;
            esac
            ;;
        darwin)
            case $ARCH in
                x86_64|amd64)
                    PLATFORM="darwin_amd64"
                    ;;
                arm64)
                    PLATFORM="darwin_arm64"
                    ;;
                *)
                    echo "Unsupported architecture: $ARCH"
                    exit 1
                    ;;
            esac
            ;;
        msys*|mingw*|cygwin*)
            PLATFORM="windows_amd64"
            ;;
        *)
            echo "Unsupported OS: $OS"
            exit 1
            ;;
    esac
    
    echo "Detected platform: $PLATFORM"
}

# Download and install
install() {
    echo "Installing androideai-core..."
    
    # Create install directory
    mkdir -p "$INSTALL_DIR"
    
    # Get latest version
    VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$VERSION" ]; then
        echo "Failed to get latest version. Building from source..."
        install_from_source
        return
    fi
    
    echo "Latest version: $VERSION"
    
    # Construct download URL
    BINARY_NAME="${BINARY}_${PLATFORM}"
    if [[ "$PLATFORM" == *"windows"* ]]; then
        BINARY_NAME="${BINARY_NAME}.exe"
    fi
    
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY_NAME"
    
    echo "Downloading from: $DOWNLOAD_URL"
    
    # Download binary
    curl -fsSL "$DOWNLOAD_URL" -o "$INSTALL_DIR/$BINARY"
    
    # Make executable
    chmod +x "$INSTALL_DIR/$BINARY"
    
    echo "Installation complete!"
    echo ""
    echo "Binary installed to: $INSTALL_DIR/$BINARY"
    echo ""
    echo "Add to your PATH if not already:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
    echo "Verify installation:"
    echo "  $BINARY --help"
}

# Install from source
install_from_source() {
    echo "Installing from source..."
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        echo "Go is not installed."
        echo "Please install Go from: https://go.dev/dl/"
        echo "Or use your package manager:"
        echo "  Ubuntu/Debian: sudo apt install golang"
        echo "  macOS: brew install go"
        echo "  Arch: sudo pacman -S go"
        exit 1
    fi
    
    echo "Go found: $(go version)"
    
    # Clone and build
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT
    
    echo "Cloning repository..."
    git clone --depth 1 "https://github.com/$REPO.git" "$TMP_DIR"
    
    cd "$TMP_DIR"
    
    echo "Building..."
    CGO_ENABLED=0 go build -tags no_treesitter -ldflags="-s -w" -o "$INSTALL_DIR/$BINARY" .
    
    echo "Installation complete!"
    echo ""
    echo "Binary installed to: $INSTALL_DIR/$BINARY"
    echo ""
    echo "Add to your PATH if not already:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
    echo "Verify installation:"
    echo "  $BINARY --help"
}

# Uninstall
uninstall() {
    echo "Uninstalling androideai-core..."
    
    if [ -f "$INSTALL_DIR/$BINARY" ]; then
        rm "$INSTALL_DIR/$BINARY"
        echo "Removed: $INSTALL_DIR/$BINARY"
    else
        echo "Binary not found at: $INSTALL_DIR/$BINARY"
    fi
    
    echo "Uninstall complete!"
}

# Main
main() {
    case "${1:-install}" in
        install)
            detect_platform
            install
            ;;
        uninstall)
            uninstall
            ;;
        from-source)
            detect_platform
            install_from_source
            ;;
        *)
            echo "Usage: $0 {install|uninstall|from-source}"
            exit 1
            ;;
    esac
}

main "$@"
