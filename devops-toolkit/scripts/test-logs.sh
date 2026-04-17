#!/bin/bash
# Test suite for DevOps Toolkit logging system
# Tests all storage backends: local, elasticsearch, loki

set -e

BASE_URL="${BASE_URL:-http://localhost:3000}"
TIMEOUT="${TIMEOUT:-5}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

passed=0
failed=0

info() { echo -e "${YELLOW}[INFO]${NC} $1"; }
pass() { echo -e "${GREEN}[PASS]${NC} $1"; passed=$((passed + 1)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; failed=$((failed + 1)); }

wait_server() {
    info "Waiting for server at $BASE_URL..."
    for i in $(seq 1 30); do
        if curl -sf "$BASE_URL/health" > /dev/null 2>&1; then
            info "Server is ready"
            return 0
        fi
        sleep 1
    done
    fail "Server did not become ready in time"
    exit 1
}

# ===========================================
# Test: Health Check
# ===========================================
test_health() {
    info "Testing health endpoint..."
    result=$(curl -sf "$BASE_URL/health")
    if [ $? -eq 0 ]; then
        pass "Health check passed"
    else
        fail "Health check failed"
    fi
}

# ===========================================
# Test: Log Retention API
# ===========================================
test_retention_get() {
    info "Testing GET /api/logs/retention..."
    result=$(curl -sf "$BASE_URL/api/logs/retention")
    if echo "$result" | grep -q '"success":true'; then
        pass "GET retention config"
    else
        fail "GET retention config"
    fi
}

test_retention_update() {
    info "Testing PUT /api/logs/retention..."
    result=$(curl -sf -X PUT "$BASE_URL/api/logs/retention" \
        -H "Content-Type: application/json" \
        -d '{"retention_days":60}')
    if echo "$result" | grep -q '"retention_days":60'; then
        pass "PUT retention update"
    else
        fail "PUT retention update"
    fi
}

test_retention_apply() {
    info "Testing POST /api/logs/retention/apply..."
    result=$(curl -sf -X POST "$BASE_URL/api/logs/retention/apply")
    if echo "$result" | grep -q '"success":true'; then
        pass "POST retention apply"
    else
        fail "POST retention apply"
    fi
}

# ===========================================
# Test: Backend Health
# ===========================================
test_backend_health() {
    info "Testing GET /api/logs/backend..."
    result=$(curl -sf "$BASE_URL/api/logs/backend")
    if echo "$result" | grep -q '"healthy":true'; then
        pass "Backend health check"
    else
        fail "Backend health check"
    fi
}

# ===========================================
# Test: Log CRUD
# ===========================================
test_log_create() {
    info "Testing POST /api/logs..."
    result=$(curl -sf -X POST "$BASE_URL/api/logs" \
        -H "Content-Type: application/json" \
        -d '{"level":"info","message":"Test log entry","source":"test-script"}')
    if echo "$result" | grep -q '"success":true'; then
        pass "Create log entry"
    else
        fail "Create log entry"
    fi
}

test_log_query() {
    info "Testing GET /api/logs..."
    result=$(curl -sf "$BASE_URL/api/logs?limit=5")
    if echo "$result" | grep -q '"success":true'; then
        pass "Query logs"
    else
        fail "Query logs"
    fi
}

test_log_stats() {
    info "Testing GET /api/logs/stats..."
    result=$(curl -sf "$BASE_URL/api/logs/stats")
    if echo "$result" | grep -q '"success":true'; then
        pass "Get log stats"
    else
        fail "Get log stats"
    fi
}

# ===========================================
# Test: Log Generation
# ===========================================
test_log_generate() {
    info "Testing POST /api/logs/generate..."
    result=$(curl -sf -X POST "$BASE_URL/api/logs/generate" \
        -H "Content-Type: application/json" \
        -d '{"count":20}')
    if echo "$result" | grep -q '"generated":20'; then
        pass "Generate sample logs"
    else
        fail "Generate sample logs"
    fi
}

# ===========================================
# Test: Alert Rules
# ===========================================
test_alert_create() {
    info "Testing POST /api/logs/alerts..."
    result=$(curl -sf -X POST "$BASE_URL/api/logs/alerts" \
        -H "Content-Type: application/json" \
        -d '{"name":"Test Alert","level":"error","pattern":"test.*error"}')
    if echo "$result" | grep -q '"success":true'; then
        pass "Create alert rule"
    else
        fail "Create alert rule"
    fi
}

test_alert_list() {
    info "Testing GET /api/logs/alerts..."
    result=$(curl -sf "$BASE_URL/api/logs/alerts")
    if echo "$result" | grep -q '"success":true'; then
        pass "List alert rules"
    else
        fail "List alert rules"
    fi
}

# ===========================================
# Test: Filters
# ===========================================
test_filter_save() {
    info "Testing POST /api/logs/filters..."
    result=$(curl -sf -X POST "$BASE_URL/api/logs/filters" \
        -H "Content-Type: application/json" \
        -d '{"name":"Test Filter","query":{"level":"error"}}')
    if echo "$result" | grep -q '"success":true'; then
        pass "Save filter"
    else
        fail "Save filter"
    fi
}

# ===========================================
# Test: Device API (sanity)
# ===========================================
test_devices_list() {
    info "Testing GET /api/devices..."
    result=$(curl -sf "$BASE_URL/api/devices")
    if echo "$result" | grep -q '"success":true'; then
        pass "List devices"
    else
        fail "List devices"
    fi
}

# ===========================================
# Test: Pipeline API (sanity)
# ===========================================
test_pipelines_list() {
    info "Testing GET /api/pipelines..."
    result=$(curl -sf "$BASE_URL/api/pipelines")
    if echo "$result" | grep -q '"success":true'; then
        pass "List pipelines"
    else
        fail "List pipelines"
    fi
}

# ===========================================
# Main
# ===========================================
main() {
    echo "=========================================="
    echo " DevOps Toolkit - Log API Test Suite"
    echo "=========================================="
    echo "Base URL: $BASE_URL"
    echo ""

    wait_server
    echo ""

    test_health
    test_retention_get
    test_retention_update
    test_retention_apply
    test_backend_health
    test_log_create
    test_log_query
    test_log_stats
    test_log_generate
    test_alert_create
    test_alert_list
    test_filter_save
    test_devices_list
    test_pipelines_list

    echo ""
    echo "=========================================="
    echo " Results: $passed passed, $failed failed"
    echo "=========================================="

    if [ $failed -gt 0 ]; then
        exit 1
    fi
    exit 0
}

main "$@"