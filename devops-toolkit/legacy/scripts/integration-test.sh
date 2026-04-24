#!/bin/bash
# Integration test runner for DevOps Toolkit

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PORT=3099
BASE_URL="http://localhost:$PORT"
PID_FILE="/tmp/devops-test-server.pid"

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  DevOps Toolkit - Integration Tests${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo ""

# Function to cleanup
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            kill "$PID" 2>/dev/null || true
            sleep 1
            kill -9 "$PID" 2>/dev/null || true
        fi
        rm -f "$PID_FILE"
    fi
    # Kill any remaining test servers on our port
    pkill -f "node.*$PORT" 2>/dev/null || true
}

trap cleanup EXIT

# Start the server
echo -e "${YELLOW}Starting test server on port $PORT...${NC}"

cd devops-toolkit
PORT=$PORT node server.js &
SERVER_PID=$!
cd ..

echo $SERVER_PID > "$PID_FILE"

# Wait for server to start
echo "Waiting for server to start..."
for i in {1..30}; do
    if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}Server started successfully!${NC}"
        break
    fi
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        echo -e "${RED}Server failed to start!${NC}"
        cat /tmp/devops-test-server.log 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

# Run integration tests
echo ""
echo -e "${YELLOW}Running integration tests...${NC}"
echo ""

# Test counter
PASSED=0
FAILED=0

# Helper function
run_test() {
    local name=$1
    local expected=$2
    shift 2
    local cmd="$@"

    echo -n "  Testing: $name... "

    result=$(eval "$cmd" 2>&1)
    actual=$?

    if [ $actual -eq $expected ]; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++))
    else
        echo -e "${RED}FAIL${NC}"
        echo "    Expected exit code: $expected, Got: $actual"
        echo "    Output: $result"
        ((FAILED++))
    fi
}

# Health check
run_test "Health endpoint" 0 \
    "curl -s $BASE_URL/health | grep -q healthy"

# Device API
echo ""
echo "Device API Tests:"

run_test "GET /api/devices" 0 \
    "curl -s $BASE_URL/api/devices | grep -q success"

run_test "POST /api/devices (register)" 0 \
    "curl -s -X POST $BASE_URL/api/devices \
        -H 'Content-Type: application/json' \
        -d '{\"type\":\"container\",\"name\":\"test-container\"}' \
        | grep -q success"

run_test "GET /api/devices/search" 0 \
    "curl -s '$BASE_URL/api/devices/search?tags=env:dev' \
        | grep -q success"

# Pipeline API
echo ""
echo "Pipeline API Tests:"

run_test "GET /api/pipelines" 0 \
    "curl -s $BASE_URL/api/pipelines | grep -q success"

run_test "GET /api/runs" 0 \
    "curl -s $BASE_URL/api/runs | grep -q success"

# Log API
echo ""
echo "Log API Tests:"

run_test "GET /api/logs" 0 \
    "curl -s $BASE_URL/api/logs | grep -q success"

run_test "GET /api/logs/stats" 0 \
    "curl -s $BASE_URL/api/logs/stats | grep -q success"

run_test "POST /api/logs (add log)" 0 \
    "curl -s -X POST $BASE_URL/api/logs \
        -H 'Content-Type: application/json' \
        -d '{\"level\":\"info\",\"message\":\"test log\"}' \
        | grep -q success"

run_test "GET /api/logs/backend" 0 \
    "curl -s $BASE_URL/api/logs/backend | grep -q success"

# Metrics API
echo ""
echo "Metrics API Tests:"

run_test "GET /metrics (Prometheus)" 0 \
    "curl -s $BASE_URL/metrics | grep -q devops_toolkit"

run_test "GET /api/metrics" 0 \
    "curl -s $BASE_URL/api/metrics | grep -q counters"

# Alert API
echo ""
echo "Alert API Tests:"

run_test "GET /api/alerts/channels" 0 \
    "curl -s $BASE_URL/api/alerts/channels | grep -q channels"

run_test "GET /api/alerts/stats" 0 \
    "curl -s $BASE_URL/api/alerts/stats | grep -q success"

run_test "POST /api/alerts/trigger" 0 \
    "curl -s -X POST $BASE_URL/api/alerts/trigger \
        -H 'Content-Type: application/json' \
        -d '{\"name\":\"test-alert\",\"severity\":\"warning\",\"message\":\"test\"}' \
        | grep -q success"

# Summary
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Test Summary${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "  ${GREEN}Passed: $PASSED${NC}"
echo -e "  ${RED}Failed: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All integration tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some integration tests failed!${NC}"
    exit 1
fi
