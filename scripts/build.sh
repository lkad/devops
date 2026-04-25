#!/bin/bash
# Build the DevOps Toolkit binary

set -e

BINARY_NAME="${BINARY_NAME:-devops-toolkit}"
OUTPUT_DIR="${OUTPUT_DIR:-.}"

echo "Building ${BINARY_NAME}..."
go build -o "${OUTPUT_DIR}/${BINARY_NAME}" ./cmd/devops-toolkit

echo "Build complete: ${OUTPUT_DIR}/${BINARY_NAME}"
