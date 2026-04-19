#!/usr/bin/env bash
# Themis Installation Script
# curl -fsSL https://raw.githubusercontent.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/main/scripts/install.sh | bash

set -e

REPO="syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey"
BIN_NAME="themis"
INSTALL_DIR="$HOME/.themis/bin"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Installing Themis ===${NC}"

# 1. Detect OS and Architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux)  PLATFORM="linux" ;;
    Darwin) PLATFORM="darwin" ;;
    *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
    x86_64|amd64) TARGET_ARCH="amd64" ;;
    arm64|aarch64) TARGET_ARCH="arm64" ;;
    *)            echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo -e "Detected platform: ${PLATFORM}-${TARGET_ARCH}"

# 2. Setup Directories
mkdir -p "$INSTALL_DIR"

# 3. Download the binary
# Note: Adjust the download URL match your actual GitHub release asset naming scheme.
# Often it looks like: themis_linux_amd64.tar.gz
ASSET_NAME="${BIN_NAME}_${PLATFORM}_${TARGET_ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
TMP_DIR=$(mktemp -d)

echo -e "Downloading latest release..."
# Download and extract the binary
if curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ASSET_NAME"; then
    tar -xzf "$TMP_DIR/$ASSET_NAME" -C "$TMP_DIR"
    # Depending on how the tar is structured, the binary might be in a folder or direct.
    # Assuming the binary 'themis' is directly extracted:
    mv "$TMP_DIR/${BIN_NAME}" "$INSTALL_DIR/${BIN_NAME}"
    chmod +x "$INSTALL_DIR/${BIN_NAME}"
else
    echo -e "${YELLOW}Warning: Precompiled binary not found at $DOWNLOAD_URL.${NC}"
    echo -e "Falling back to 'go install' if Go is installed..."
    if command -v go >/dev/null 2>&1; then
        go install github.com/${REPO}@latest
        # go install places it in $GOPATH/bin, copy it to our uniform dir
        GOPATH=$(go env GOPATH)
        cp "$GOPATH/bin/${BIN_NAME}" "$INSTALL_DIR/${BIN_NAME}"
    else
        echo -e "Error: Go is not installed. Cannot build from source."
        exit 1
    fi
fi

rm -rf "$TMP_DIR"

# 4. Export to PATH for Bash, Zsh, and Fish
inject_path() {
    local rc_file="$1"
    local path_cmd="$2"
    if [ -f "$rc_file" ]; then
        if ! grep -q "$INSTALL_DIR" "$rc_file"; then
            echo -e "\n$path_cmd" >> "$rc_file"
            echo "Added Themis to $rc_file"
        fi
    fi
}

inject_path "$HOME/.bashrc" "export PATH=\"\$PATH:$INSTALL_DIR\""
inject_path "$HOME/.zshrc" "export PATH=\"\$PATH:$INSTALL_DIR\""

# Fish configuration
FISH_CONFIG_DIR="$HOME/.config/fish/conf.d"
if command -v fish >/dev/null 2>&1; then
    mkdir -p "$FISH_CONFIG_DIR"
    if [ ! -f "$FISH_CONFIG_DIR/themis.fish" ]; then
        echo "set -gx PATH \$PATH $INSTALL_DIR" > "$FISH_CONFIG_DIR/themis.fish"
        echo "Added Themis to fish config"
    fi
fi

# 5. Success
echo -e "${GREEN}✔ Themis was successfully installed to $INSTALL_DIR${NC}"
echo -e "\nTo get started, please restart your terminal or run:"
echo -e "  ${YELLOW}export PATH=\"\$PATH:$INSTALL_DIR\"${NC}"
echo -e "\nThen run: ${BLUE}themis --help${NC}"
