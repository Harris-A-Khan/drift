#!/bin/bash
# Drift CLI Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Harris-A-Khan/drift/main/install.sh | bash

set -e

REPO="Harris-A-Khan/drift"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="drift"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    darwin|linux)
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

ASSET_NAME="drift-${OS}-${ARCH}"

echo "Installing drift for ${OS}/${ARCH}..."

# Get latest release URL
DOWNLOAD_URL=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep "browser_download_url.*${ASSET_NAME}\"" | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Error: Could not find release for ${ASSET_NAME}"
    exit 1
fi

# Download binary
TMP_FILE=$(mktemp)
echo "Downloading from ${DOWNLOAD_URL}..."
curl -sL -o "$TMP_FILE" "$DOWNLOAD_URL"
chmod +x "$TMP_FILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo "drift installed successfully!"
echo ""
drift --version
echo ""
echo "Get started:"
echo "  drift init      - Initialize drift in your project"
echo "  drift doctor    - Check system dependencies"
echo "  drift --help    - Show all commands"
