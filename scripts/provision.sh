#!/bin/bash
# Homebase — One-shot Raspberry Pi provisioning
# Run on a fresh Raspberry Pi OS (64-bit) install
set -euo pipefail

REPO="emm5317/homebase"
INSTALL_DIR="/opt/homebase"

echo "=== Homebase Pi Provisioning ==="

# 1. System updates
echo "[1/9] Updating system..."
sudo apt update && sudo apt upgrade -y

# 2. Disable swap (reduce SD card writes)
echo "[2/9] Disabling swap..."
sudo dphys-swapfile swapoff 2>/dev/null || true
sudo systemctl disable dphys-swapfile 2>/dev/null || true

# 3. Set hostname
echo "[3/9] Setting hostname to 'homebase'..."
sudo hostnamectl set-hostname homebase

# 4. Create service user and directories
echo "[4/9] Creating service user and directories..."
sudo useradd -r -s /usr/sbin/nologin homebase 2>/dev/null || true
sudo useradd -r -s /usr/sbin/nologin cloudflared 2>/dev/null || true
sudo mkdir -p "$INSTALL_DIR/data"
sudo chown homebase:homebase "$INSTALL_DIR/data"

# 5. Install cloudflared
echo "[5/9] Installing cloudflared..."
ARCH=$(dpkg --print-architecture)
curl -sL "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${ARCH}" -o /tmp/cloudflared
sudo mv /tmp/cloudflared /usr/local/bin/cloudflared
sudo chmod +x /usr/local/bin/cloudflared

# 6. Install litestream
echo "[6/9] Installing litestream..."
LITESTREAM_VERSION="v0.3.13"
curl -sL "https://github.com/benbjohnson/litestream/releases/download/${LITESTREAM_VERSION}/litestream-${LITESTREAM_VERSION}-linux-${ARCH}.tar.gz" | sudo tar -xz -C /usr/local/bin

# 7. Download latest homebase binary
echo "[7/9] Downloading homebase..."
LATEST=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4)
if [ -n "$LATEST" ]; then
  curl -sL "https://github.com/$REPO/releases/download/$LATEST/homebase-linux-arm64" -o /tmp/homebase
  sudo mv /tmp/homebase "$INSTALL_DIR/homebase"
  sudo chmod +x "$INSTALL_DIR/homebase"
  echo "  Installed homebase $LATEST"
else
  echo "  WARNING: Could not find latest release. Copy the binary manually to $INSTALL_DIR/homebase"
fi

# 8. Install systemd services
echo "[8/9] Installing systemd services..."
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR/../deploy"

sudo cp "$DEPLOY_DIR/homebase.service" /etc/systemd/system/
sudo cp "$DEPLOY_DIR/cloudflared.service" /etc/systemd/system/
sudo cp "$DEPLOY_DIR/litestream.service" /etc/systemd/system/

# 9. Configure tmpfs for reduced SD card writes
echo "[9/9] Configuring tmpfs..."
if ! grep -q "tmpfs /tmp" /etc/fstab; then
  echo "tmpfs /tmp tmpfs defaults,noatime,nosuid,size=100m 0 0" | sudo tee -a /etc/fstab
  echo "tmpfs /var/log tmpfs defaults,noatime,nosuid,size=50m 0 0" | sudo tee -a /etc/fstab
fi

# Add noatime to root filesystem if not already set
sudo sed -i 's/defaults/defaults,noatime/' /etc/fstab 2>/dev/null || true

echo ""
echo "=== Provisioning complete ==="
echo ""
echo "Next steps:"
echo "  1. Copy your config:   sudo cp config.example.yaml $INSTALL_DIR/config.yaml"
echo "  2. Edit config:        sudo nano $INSTALL_DIR/config.yaml"
echo "  3. Create .env:        sudo nano $INSTALL_DIR/.env"
echo "  4. Set up tunnel:      cloudflared tunnel create homebase"
echo "  5. Copy tunnel config: sudo cp deploy/cloudflared.yml /etc/cloudflared/config.yml"
echo "  6. Copy litestream:    sudo cp deploy/litestream.yml /etc/litestream.yml"
echo "  7. Enable services:"
echo "     sudo systemctl daemon-reload"
echo "     sudo systemctl enable homebase cloudflared litestream"
echo "     sudo systemctl start homebase cloudflared litestream"
echo "  8. Open http://homebase.local:8080 on your tablet"
