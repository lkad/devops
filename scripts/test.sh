#!/bin/bash
# Run tests for DevOps Toolkit

set -e

PACKAGE="${1:-./...}"
VERBOSE="${VERBOSE:-}"

echo "Running tests for: ${PACKAGE}"
if [ -n "${VERBOSE}" ]; then
    go test -v ${PACKAGE}
else
    go test ${PACKAGE}
fi

echo "Tests complete."
