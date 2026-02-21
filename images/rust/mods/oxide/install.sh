#!/bin/bash
# Oxide (uMod) install script for Rust
# Downloads and installs the latest Oxide release from GitHub

echo "-> Downloading latest Oxide for Rust..."

# Get the latest release download URL from GitHub API
OXIDE_URL=$(curl -s https://api.github.com/repos/OxideMod/Oxide.Rust/releases/latest \
    | grep "browser_download_url.*Oxide.Rust-linux.zip" \
    | cut -d '"' -f 4)

if [[ -z "$OXIDE_URL" ]]; then
    echo "   Warning: Failed to get Oxide download URL from GitHub"
    exit 0  # Don't fail startup, just skip Oxide
fi

echo "   Downloading from: $OXIDE_URL"

# Download Oxide
if ! curl -sL -o /tmp/oxide.zip "$OXIDE_URL"; then
    echo "   Warning: Failed to download Oxide"
    exit 0
fi

# Extract to server directory (overwrites existing files)
echo "   Extracting Oxide to server..."
if ! unzip -o /tmp/oxide.zip -d /data/server/RustDedicated_Data/ > /dev/null 2>&1; then
    echo "   Warning: Failed to extract Oxide"
    rm -f /tmp/oxide.zip
    exit 0
fi

# Cleanup
rm -f /tmp/oxide.zip

echo "   Oxide installed successfully"
