#!/bin/bash
# Run the DevOps Toolkit server

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

BINARY_NAME="${BINARY_NAME:-devops-toolkit}"
CONFIG="${CONFIG:-config.yaml}"

if [ ! -f "./${BINARY_NAME}" ]; then
    echo "Binary not found. Run scripts/build.sh first."
    exit 1
fi

# Loki configuration for historical logs
export LOKI_URL="${LOKI_URL:-http://localhost:3100}"
export LOG_STORAGE_BACKEND="${LOG_STORAGE_BACKEND:-loki}"

echo "Starting ${BINARY_NAME} with config: ${CONFIG}"
echo "Loki URL: ${LOKI_URL}, Backend: ${LOG_STORAGE_BACKEND}"
./${BINARY_NAME}
