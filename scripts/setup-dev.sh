#!/bin/bash
# Set up local development environment for DevOps Toolkit

set -e

echo "Setting up DevOps Toolkit development environment..."

# Check for required tools
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Aborting." >&2; exit 1; }

# Build the binary
echo "Building binary..."
./scripts/build.sh

# Check if k3d is available and create test cluster
if command -v k3d >/dev/null 2>&1; then
    echo "k3d found. Creating test cluster..."
    k3d cluster create dev-cluster-1 --agents 3 -p "31000:30000@server:0" || true
    echo "Test cluster created (or already exists)."
else
    echo "k3d not found. Skipping cluster creation."
    echo "To install k3d: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash"
fi

# Check if Docker is available for LDAP and PostgreSQL
if command -v docker >/dev/null 2>&1; then
    echo "Docker found. Starting test services..."
    (cd /mnt/devops && docker compose up -d) || true
    echo "LDAP and PostgreSQL started."
else
    echo "Docker not found. Skipping LDAP and PostgreSQL setup."
fi

echo ""
echo "Development environment ready!"
echo ""
echo "Next steps:"
echo "  1. Configure PostgreSQL connection in config.yaml"
echo "  2. Run: ./scripts/run.sh"
echo "  3. Verify: curl http://localhost:3000/health"
