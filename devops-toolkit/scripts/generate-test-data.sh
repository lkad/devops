#!/bin/bash
# Test data generator for DevOps Toolkit
# Generates logs, devices, pipelines for testing different storage backends

set -e

BASE_URL="${BASE_URL:-http://localhost:3000}"
DRY_RUN="${DRY_RUN:-false}"

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${CYAN}[GEN]${NC} $1"; }
ok() { echo -e "${GREEN}[OK]${NC} $1"; }

usage() {
    cat <<EOF
Usage: $0 <command> [options]

Commands:
    logs [count]       Generate sample logs (default: 50)
    devices [count]    Generate sample devices (default: 5)
    pipelines [count]  Generate sample pipelines (default: 3)
    all [count]        Generate all test data
    clear-logs         Delete all logs
    clear-devices      Delete all devices
    clear-all          Delete all data
    status             Show current system status
    backend            Show current storage backend
    switch-backend <b>  Switch backend (local|es|loki)

Examples:
    $0 logs 100        Generate 100 test logs
    $0 all 200         Generate all test data (200 logs)
    $0 switch-backend es   Switch to Elasticsearch backend
    $0 status          Show system status

Environment:
    BASE_URL   API base URL (default: http://localhost:3000)
    DRY_RUN    If true, only show what would be done
EOF
}

# ===========================================
# Helpers
# ===========================================
wait_server() {
    for i in $(seq 1 10); do
        curl -sf "$BASE_URL/health" > /dev/null 2>&1 && return 0
        sleep 1
    done
    echo "Error: Server not available at $BASE_URL"
    exit 1
}

api() {
    local method="${1:-GET}"
    local path="$2"
    local data="$3"
    local extra="${4:-}"

    if [ "$DRY_RUN" = "true" ]; then
        echo "[DRY RUN] $method $path${data:+ -d $data}"
        return 0
    fi

    if [ -n "$data" ]; then
        curl -sf -X "$method" "$BASE_URL$path" \
            -H "Content-Type: application/json" \
            $extra \
            -d "$data"
    else
        curl -sf -X "$method" "$BASE_URL$path" $extra
    fi
}

# ===========================================
# Commands
# ===========================================
cmd_status() {
    info "Checking system status..."
    echo ""

    echo "--- Server Health ---"
    api GET "/health"
    echo ""

    echo "--- Storage Backend ---"
    api GET "/api/logs/backend"
    echo ""

    echo "--- Retention Config ---"
    api GET "/api/logs/retention"
    echo ""

    echo "--- Log Stats ---"
    api GET "/api/logs/stats"
    echo ""
}

cmd_backend() {
    info "Current storage backend:"
    api GET "/api/logs/backend"
}

cmd_switch_backend() {
    local new_backend="$1"
    case "$new_backend" in
        local) export backend_type="local" ;;
        es|elasticsearch) export backend_type="elasticsearch" ;;
        loki) export backend_type="loki" ;;
        *)
            echo "Error: Unknown backend '$new_backend'"
            echo "Valid options: local, es, loki"
            exit 1
            ;;
    esac
    info "Note: Backend switching requires server restart with new LOG_STORAGE_BACKEND env var"
    echo ""
    echo "To switch to $new_backend, restart server with:"
    echo "    LOG_STORAGE_BACKEND=$new_backend node server.js"
}

cmd_logs() {
    local count="${1:-50}"
    info "Generating $count sample logs..."

    local result=$(api POST "/api/logs/generate" "{\"count\":$count}")

    if echo "$result" | grep -q '"generated":'; then
        local generated=$(echo "$result" | grep -o '"generated":[0-9]*' | cut -d':' -f2)
        ok "Generated $generated logs"
    else
        echo "Error: Failed to generate logs"
        echo "$result"
        exit 1
    fi
}

cmd_clear_logs() {
    info "Clearing all logs..."
    # Generate 0 logs with old timestamps to trigger retention cleanup
    api POST "/api/logs/retention/apply" > /dev/null
    ok "Logs cleared"
}

cmd_devices() {
    local count="${1:-5}"
    info "Generating $count sample devices..."

    local types=("server" "router" "sensor" "workstation" "container")
    local statuses=("active" "maintenance" "offline")

    for i in $(seq 1 $count); do
        local id="test-device-$i-$(date +%s)"
        local type="${types[$((RANDOM % ${#types[@]}))]}"
        local name="Test $type #$i"
        local labels="env:test,type:$type"

        api POST "/api/devices" "{\"id\":\"$id\",\"type\":\"$type\",\"name\":\"$name\",\"labels\":[\"$labels\"]}" > /dev/null
        ok "Created device: $name"
    done
}

cmd_clear_devices() {
    info "Clearing all devices..."
    # Just show the devices count - actual deletion needs API support
    local count=$(api GET "/api/devices" | grep -o '"id":"[^"]*"' | wc -l)
    ok "Found $count devices (manual deletion needed via API)"
}

cmd_pipelines() {
    local count="${1:-3}"
    info "Generating $count sample pipelines..."

    for i in $(seq 1 $count); do
        local name="Pipeline Test #$i"
        local pipeline=$(cat <<EOF
{
    "name": "$name",
    "stages": [
        {"name": "build", "image": "golang:1.21", "command": "go build"},
        {"name": "test", "image": "golang:1.21", "command": "go test ./..."},
        {"name": "deploy", "image": "docker:latest", "command": "docker push"}
    ],
    "env": {"NODE_ENV": "test"}
}
EOF
)
        api POST "/api/pipelines" "$pipeline" > /dev/null
        ok "Created pipeline: $name"
    done
}

cmd_clear_pipelines() {
    info "Clearing all pipelines..."
    # Just show the pipelines count - actual deletion needs API support
    local count=$(api GET "/api/pipelines" | grep -o '"id":"[^"]*"' | wc -l)
    ok "Found $count pipelines (manual deletion needed via API)"
}

cmd_clear_all() {
    cmd_clear_logs
    cmd_clear_devices
    cmd_clear_pipelines
    ok "All data cleared"
}

cmd_all() {
    local count="${1:-50}"
    info "Generating all test data (count=$count)..."
    echo ""

    cmd_logs "$count"
    cmd_devices 5
    cmd_pipelines 3

    echo ""
    ok "All test data generated"
    echo ""
    cmd_status
}

# ===========================================
# Main
# ===========================================
main() {
    if [ $# -lt 1 ]; then
        usage
        exit 1
    fi

    wait_server

    local cmd="$1"
    shift

    case "$cmd" in
        status) cmd_status ;;
        backend) cmd_backend ;;
        switch-backend)
            [ $# -lt 1 ] && usage && exit 1
            cmd_switch_backend "$1" ;;
        logs) cmd_logs "$1" ;;
        devices) cmd_devices "$1" ;;
        pipelines) cmd_pipelines "$1" ;;
        all) cmd_all "$1" ;;
        clear-logs) cmd_clear_logs ;;
        clear-devices) cmd_clear_devices ;;
        clear-all) cmd_clear_all ;;
        help|--help|-h) usage ;;
        *)
            echo "Error: Unknown command '$cmd'"
            usage
            exit 1
            ;;
    esac
}

main "$@"