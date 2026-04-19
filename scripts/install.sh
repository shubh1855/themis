#!/usr/bin/env bash
set -e

# ==============================================================================
# Themis CLI - Linux & macOS Installer
# ==============================================================================
# Since the repository is private, you need a GitHub PAT, or you can change
# the GITHUB_API_URL below to point to your Cloudflare Worker Proxy.
#
# Usage: 
#   curl -sL https://public-host.com/install.sh | bash -s -- YOUR_GITHUB_TOKEN
# ==============================================================================

REPO="syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey"
BINARY_NAME="themis"
INSTALL_DIR="/usr/local/bin"

echo -e "\033[1;36mInstalling $BINARY_NAME CLI...\033[0m"

GITHUB_TOKEN="${GITHUB_TOKEN:-$1}"
if [ -z "$GITHUB_TOKEN" ]; then
    echo -e "\033[1;31m❌ Error: Private repository requires a GitHub token.\033[0m"
    echo "Usage: curl -sL <url> | bash -s -- YOUR_TOKEN"
    echo "Alternatively, deploy the Cloudflare Worker proxy to download without tokens!"
    exit 1
fi

echo "🔍 Fetching latest release from GitHub (Private repo)..."
LATEST_RELEASE=$(curl -sH "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$REPO/releases/latest")

# Detect Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "💻 Detected Platform: $OS-$ARCH"

# Find correct asset URL natively
ASSET_URL=$(echo "$LATEST_RELEASE" | grep -o 'https://api.github.com/repos/[^"]*assets/[^"]*' | grep -i "$OS" | grep -i "$ARCH" | head -n 1)

if [ -z "$ASSET_URL" ]; then
    echo -e "\033[1;31m❌ Could not find a suitable release asset for $OS $ARCH.\033[0m"
    exit 1
fi

echo "⬇️ Downloading latest release tarball..."
TEMP_TAR="/tmp/$BINARY_NAME.tar.gz"
curl -sL -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/octet-stream" "$ASSET_URL" -o "$TEMP_TAR"

echo "📦 Extracting and moving to $INSTALL_DIR (will request sudo)..."
TMP_EXTRACT="/tmp/${BINARY_NAME}_extract"
mkdir -p "$TMP_EXTRACT"
tar -xzf "$TEMP_TAR" -C "$TMP_EXTRACT"

# Find binary inside the extracted folder
EXTRACTED_BIN=$(find "$TMP_EXTRACT" -type f -name "$BINARY_NAME" | head -n 1)

if [ -z "$EXTRACTED_BIN" ]; then
    echo -e "\033[1;31m❌ Could not find $BINARY_NAME binary inside the extracted archive.\033[0m"
    exit 1
fi

sudo mv "$EXTRACTED_BIN" "$INSTALL_DIR/$BINARY_NAME"
sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"

rm -rf "$TMP_EXTRACT" "$TEMP_TAR"

echo -e "\033[1;32m✅ $BINARY_NAME installed successfully! Run '$BINARY_NAME' to start bridging agents.\033[0m"
