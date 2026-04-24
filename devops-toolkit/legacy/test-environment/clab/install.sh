#!/bin/bash
# Containerlab Installation Script
# ================================
# This script installs Containerlab and sets up the environment

set -e

echo "=========================================="
echo "   Containerlab Installation Script"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root or with sudo"
    exit 1
fi

# Detect OS
OS=$(uname -s)
if [ "$OS" != "Linux" ]; then
    echo "This script only supports Linux"
    exit 1
fi

echo "[1/4] Installing Containerlab..."
curl -sL https://get.containerlab.dev | bash

echo ""
echo "[2/4] Verifying installation..."
clab version

echo ""
echo "[3/4] Adding current user to clab_admins group..."
TARGET_USER=${SUDO_USER:-$USER}
usermod -aG clab_admins "$TARGET_USER"
echo "User $TARGET_USER added to clab_admins group"
echo "Please run 'newgrp clab_admins' or logout/login for changes to take effect"

echo ""
echo "[4/4] Checking Docker..."
if ! command -v docker &> /dev/null; then
    echo "Docker not found. Please install Docker first."
    exit 1
fi
docker version --format '{{.Server.Version}}' || echo "Docker daemon may not be running"

echo ""
echo "=========================================="
echo "   Installation Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Run: newgrp clab_admins"
echo "  2. Run: cd /mnt/devops/devops-toolkit/test-environment/clab"
echo "  3. Run: sudo clab deploy -t topology.yml"
echo "  4. Run: ./clab.sh status"
echo ""
