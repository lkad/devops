#!/bin/bash
# CI test runner - tests all storage backends in sequence
# Used for automated testing in CI/CD pipelines

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SERVER_FILE="$PROJECT_DIR/server.js"
LOG_FILE="/tmp/devops-test.log"
PID=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[TEST]${NC} $1" | tee -a "$LOG_FILE"; }
pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((PASS++)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; ((FAIL++)); }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

# Counters
PASS=0
FAIL=0

cleanup() {
    if [ -n "$PID" ]; then
        kill $PID 2>/dev/null || true
    fi
    rm -f /tmp/devops-toolkit.pid
}

trap cleanup EXIT

wait_server() {
    local url="$1"
    for i in {1..30}; do
        curl -sf "$url/health" > /dev/null 2>&1 && return 0
        sleep 1
    done
    return 1
}

start_server() {
    local backend="$1"
    local port="${2:-3000}"

    log "Starting server with $backend backend on port $port..."

    export LOG_STORAGE_BACKEND="$backend"
    export PORT="$port"

    cd "$PROJECT_DIR"
    node server.js > "$LOG_FILE" 2>&1 &
    PID=$!

    echo "$PID" > /tmp/devops-toolkit.pid

    if ! wait_server "http://localhost:$port"; then
        fail "Server failed to start with $backend backend"
        cat "$LOG_FILE"
        return 1
    fi

    log "Server ready with $backend backend"
    return 0
}

run_tests() {
    local backend="$1"
    local port="${2:-3000}"

    log "=========================================="
    log " Testing Backend: $backend"
    log "=========================================="

    export BASE_URL="http://localhost:$port"

    # Run bash tests
    if bash "$SCRIPT_DIR/test-logs.sh" 2>&1 | tee -a "$LOG_FILE"; then
        pass "$backend backend - bash tests"
    else
        fail "$backend backend - bash tests"
    fi

    # Run node tests
    if node "$SCRIPT_DIR/run-tests.js" --url "$BASE_URL" 2>&1 | tee -a "$LOG_FILE"; then
        pass "$backend backend - node tests"
    else
        fail "$backend backend - node tests"
    fi

    log ""
}

# ===========================================
# Main
# ===========================================
main() {
    echo "╔═══════════════════════════════════════════╗"
    echo "║   DevOps Toolkit - CI Test Runner        ║"
    echo "╚═══════════════════════════════════════════╝"
    echo ""

    local port=3000

    for backend in local elasticsearch loki; do
        # Kill previous server
        pkill -f "node server.js" 2>/dev/null || true
        sleep 2

        if start_server "$backend" "$port"; then
            run_tests "$backend" "$port"
        fi

        # Kill server after each backend test
        pkill -f "node server.js" 2>/dev/null || true
        sleep 1

        port=$((port + 1))
    done

    echo ""
    echo "╔═══════════════════════════════════════════╗"
    echo "║   Final Results"
    echo "╠═══════════════════════════════════════════╣"
    echo "║   Passed: $PASS"
    echo "║   Failed: $FAIL"
    echo "╚═══════════════════════════════════════════╝"

    if [ $FAIL -gt 0 ]; then
        exit 1
    fi
    exit 0
}

main "$@"