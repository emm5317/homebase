#!/bin/bash
# Homebase — Update to latest release
set -euo pipefail

REPO="emm5317/homebase"
INSTALL_DIR="/opt/homebase"
BINARY="$INSTALL_DIR/homebase"

# Get current version
CURRENT=$("$BINARY" --version 2>/dev/null || echo "unknown")

# Get latest release
LATEST=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
  echo "ERROR: Could not fetch latest release"
  exit 1
fi

if [ "$LATEST" = "$CURRENT" ]; then
  echo "Already up to date ($CURRENT)"
  exit 0
fi

echo "Updating from $CURRENT to $LATEST..."

# Download new binary
ARCH=$(dpkg --print-architecture)
curl -sL "https://github.com/$REPO/releases/download/$LATEST/homebase-linux-${ARCH}" -o /tmp/homebase
chmod +x /tmp/homebase

# Stop, swap, start
sudo systemctl stop homebase
cp "$BINARY" "$BINARY.bak"
sudo mv /tmp/homebase "$BINARY"
sudo systemctl start homebase

echo "Updated to $LATEST"
echo "Rollback: sudo systemctl stop homebase && mv $BINARY.bak $BINARY && sudo systemctl start homebase"
