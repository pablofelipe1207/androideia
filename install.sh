#!/bin/bash
set -e

# androideai-core installer
# Usage: curl -fsSL https://raw.githubusercontent.com/mobiai/androideai-core/main/install.sh | bash

REPO="mobiai/androideai-core"
BINARY="androideai"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
ANDROID_SDK_DIR="${ANDROID_SDK_DIR:-$HOME/Android/Sdk}"
ANDROID_CMDLINE_TOOLS_VERSION="11076708"

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

# Check if a command exists
command_exists() {
    command -v "$1" &> /dev/null
}

# Check and install Android SDK tools
check_android_sdk() {
    echo ""
    echo "Checking Android SDK tools..."
    
    # Check if ANDROID_HOME is set
    if [ -n "$ANDROID_HOME" ]; then
        ANDROID_SDK_DIR="$ANDROID_HOME"
    fi
    
    # Check common Android SDK locations
    local SDK_FOUND=false
    for dir in "$HOME/Android/Sdk" "/usr/lib/android-sdk" "/opt/android-sdk" "$ANDROID_SDK_DIR"; do
        if [ -d "$dir" ]; then
            ANDROID_SDK_DIR="$dir"
            SDK_FOUND=true
            echo "Android SDK found at: $ANDROID_SDK_DIR"
            break
        fi
    done
    
    # Check if adb exists
    if command_exists adb; then
        echo "✅ adb found: $(which adb)"
        ADB_PATH=$(which adb)
    elif [ -f "$ANDROID_SDK_DIR/platform-tools/adb" ]; then
        echo "✅ adb found: $ANDROID_SDK_DIR/platform-tools/adb"
        ADB_PATH="$ANDROID_SDK_DIR/platform-tools/adb"
        export PATH="$ANDROID_SDK_DIR/platform-tools:$PATH"
    else
        echo "❌ adb not found"
        ADB_PATH=""
    fi
    
    # Check if emulator exists
    if command_exists emulator; then
        echo "✅ emulator found: $(which emulator)"
        EMULATOR_PATH=$(which emulator)
    elif [ -f "$ANDROID_SDK_DIR/emulator/emulator" ]; then
        echo "✅ emulator found: $ANDROID_SDK_DIR/emulator/emulator"
        EMULATOR_PATH="$ANDROID_SDK_DIR/emulator/emulator"
        export PATH="$ANDROID_SDK_DIR/emulator:$PATH"
    else
        echo "❌ emulator not found"
        EMULATOR_PATH=""
    fi
    
    # Check if sdkmanager exists
    if command_exists sdkmanager; then
        echo "✅ sdkmanager found: $(which sdkmanager)"
        SDKMANAGER_PATH=$(which sdkmanager)
    elif [ -f "$ANDROID_SDK_DIR/cmdline-tools/latest/bin/sdkmanager" ]; then
        echo "✅ sdkmanager found: $ANDROID_SDK_DIR/cmdline-tools/latest/bin/sdkmanager"
        SDKMANAGER_PATH="$ANDROID_SDK_DIR/cmdline-tools/latest/bin/sdkmanager"
        export PATH="$ANDROID_SDK_DIR/cmdline-tools/latest/bin:$PATH"
    else
        echo "⚠️  sdkmanager not found (optional)"
        SDKMANAGER_PATH=""
    fi
    
    # If essential tools are missing, offer to install them
    if [ -z "$ADB_PATH" ] || [ -z "$EMULATOR_PATH" ]; then
        echo ""
        echo "Some Android SDK tools are missing."
        read -p "Would you like to install Android SDK command-line tools? (y/N) " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            install_android_sdk
        else
            echo ""
            echo "Skipping Android SDK installation."
            echo "You can install it later manually or set ANDROID_HOME environment variable."
        fi
    fi
}

# Install Android SDK command-line tools
install_android_sdk() {
    echo ""
    echo "Installing Android SDK command-line tools..."
    
    # Create SDK directory
    mkdir -p "$ANDROID_SDK_DIR"
    
    # Detect platform for download
    local OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    local ARCH=$(uname -m)
    
    case $OS in
        linux)
            if [ "$ARCH" = "x86_64" ] || [ "$ARCH" = "amd64" ]; then
                CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-linux-${ANDROID_CMDLINE_TOOLS_VERSION}_latest.zip"
            elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
                CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-linux-${ANDROID_CMDLINE_TOOLS_VERSION}_latest.zip"
            else
                echo "Unsupported architecture for Android SDK: $ARCH"
                return 1
            fi
            ;;
        darwin)
            if [ "$ARCH" = "x86_64" ] || [ "$ARCH" = "amd64" ]; then
                CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-mac-${ANDROID_CMDLINE_TOOLS_VERSION}_latest.zip"
            elif [ "$ARCH" = "arm64" ]; then
                CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-mac-${ANDROID_CMDLINE_TOOLS_VERSION}_latest.zip"
            else
                echo "Unsupported architecture for Android SDK: $ARCH"
                return 1
            fi
            ;;
        *)
            echo "Unsupported OS for Android SDK: $OS"
            return 1
            ;;
    esac
    
    echo "Downloading Android SDK command-line tools..."
    
    # Download to temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT
    
    curl -fsSL "$CMDLINE_TOOLS_URL" -o "$TMP_DIR/commandlinetools.zip"
    
    # Extract
    echo "Extracting..."
    unzip -q "$TMP_DIR/commandlinetools.zip" -d "$TMP_DIR"
    
    # Create cmdline-tools directory
    mkdir -p "$ANDROID_SDK_DIR/cmdline-tools"
    
    # Move to latest directory
    if [ -d "$ANDROID_SDK_DIR/cmdline-tools/latest" ]; then
        rm -rf "$ANDROID_SDK_DIR/cmdline-tools/latest"
    fi
    mv "$TMP_DIR/cmdline-tools" "$ANDROID_SDK_DIR/cmdline-tools/latest"
    
    # Make tools executable
    chmod +x "$ANDROID_SDK_DIR/cmdline-tools/latest/bin/"*
    
    echo "✅ Android SDK command-line tools installed to: $ANDROID_SDK_DIR/cmdline-tools/latest"
    
    # Add to PATH for this session
    export PATH="$ANDROID_SDK_DIR/cmdline-tools/latest/bin:$ANDROID_SDK_DIR/platform-tools:$PATH"
    
    # Install required SDK components
    echo ""
    echo "Installing required SDK components..."
    
    # Accept licenses
    yes | sdkmanager --licenses > /dev/null 2>&1 || true
    
    # Install platform-tools (adb)
    echo "Installing platform-tools (adb)..."
    sdkmanager "platform-tools" > /dev/null 2>&1 || true
    
    # Install emulator
    echo "Installing emulator..."
    sdkmanager "emulator" > /dev/null 2>&1 || true
    
    # Install a system image for testing (optional)
    echo ""
    read -p "Would you like to install a system image for testing? (y/N) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Installing system image (this may take a while)..."
        sdkmanager "system-images;android-34;google_apis;x86_64" > /dev/null 2>&1 || true
        echo "Creating AVD..."
        echo "no" | avdmanager create avd -n test_avd -k "system-images;android-34;google_apis;x86_64" > /dev/null 2>&1 || true
    fi
    
    echo ""
    echo "✅ Android SDK installation complete!"
    echo ""
    echo "To use Android SDK tools, add the following to your shell profile:"
    echo "  export ANDROID_HOME=\"$ANDROID_SDK_DIR\""
    echo "  export PATH=\"\$ANDROID_HOME/cmdline-tools/latest/bin:\$ANDROID_HOME/platform-tools:\$PATH\""
    
    # Update shell profile
    update_shell_profile
}

# Update shell profile with Android SDK paths
update_shell_profile() {
    local SHELL_PROFILE=""
    
    if [ -f "$HOME/.bashrc" ]; then
        SHELL_PROFILE="$HOME/.bashrc"
    elif [ -f "$HOME/.bash_profile" ]; then
        SHELL_PROFILE="$HOME/.bash_profile"
    elif [ -f "$HOME/.zshrc" ]; then
        SHELL_PROFILE="$HOME/.zshrc"
    fi
    
    if [ -n "$SHELL_PROFILE" ]; then
        echo ""
        echo "Updating $SHELL_PROFILE..."
        
        # Check if already configured
        if grep -q "ANDROID_HOME" "$SHELL_PROFILE" 2>/dev/null; then
            echo "Android SDK paths already configured in $SHELL_PROFILE"
        else
            cat >> "$SHELL_PROFILE" << EOF

# Android SDK
export ANDROID_HOME="$ANDROID_SDK_DIR"
export PATH="\$ANDROID_HOME/cmdline-tools/latest/bin:\$ANDROID_HOME/platform-tools:\$PATH"
EOF
            echo "Added Android SDK paths to $SHELL_PROFILE"
            echo "Please restart your shell or run: source $SHELL_PROFILE"
        fi
    fi
}

# Download and install androideai
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
    
    echo "✅ androideai-core installation complete!"
    echo ""
    echo "Binary installed to: $INSTALL_DIR/$BINARY"
}

# Install from source
install_from_source() {
    echo "Installing from source..."
    
    # Check if Go is installed
    if ! command_exists go; then
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
    
    echo "✅ androideai-core installation complete!"
    echo ""
    echo "Binary installed to: $INSTALL_DIR/$BINARY"
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

# Print installation summary
print_summary() {
    echo ""
    echo "=========================================="
    echo "  Installation Summary"
    echo "=========================================="
    echo ""
    echo "Binary: $INSTALL_DIR/$BINARY"
    echo ""
    echo "To get started:"
    echo "  1. Add to your PATH (if not already):"
    echo "     export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
    echo "  2. Initialize a project:"
    echo "     androideai init"
    echo ""
    echo "  3. Import Android skills:"
    echo "     androideai skills import-android"
    echo ""
    echo "  4. Start the agent:"
    echo "     androideai agent 'Create a login feature'"
    echo ""
    echo "=========================================="
}

# Main
main() {
    case "${1:-install}" in
        install)
            detect_platform
            install
            check_android_sdk
            print_summary
            ;;
        uninstall)
            uninstall
            ;;
        from-source)
            detect_platform
            install_from_source
            check_android_sdk
            print_summary
            ;;
        sdk)
            detect_platform
            check_android_sdk
            ;;
        *)
            echo "Usage: $0 {install|uninstall|from-source|sdk}"
            echo ""
            echo "Commands:"
            echo "  install      Install androideai-core and check Android SDK"
            echo "  uninstall    Remove androideai-core"
            echo "  from-source  Build and install from source"
            echo "  sdk          Check and install Android SDK tools only"
            exit 1
            ;;
    esac
}

main "$@"
