#!/bin/bash
# Run the DevOps Toolkit server

set -e

BINARY_NAME="${BINARY_NAME:-devops-toolkit}"
CONFIG="${CONFIG:-config.yaml}"

if [ ! -f "${BINARY_NAME}" ]; then
    echo "Binary not found. Run scripts/build.sh first."
    exit 1
fi

echo "Starting ${BINARY_NAME} with config: ${CONFIG}"
./${BINARY_NAME}
