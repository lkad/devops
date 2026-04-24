#!/bin/bash
# Switch between storage backends for the DevOps Toolkit server
# Usage: ./switch-backend.sh [local|es|loki]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SERVER_FILE="$PROJECT_DIR/server.js"
PID_FILE="/tmp/devops-toolkit.pid"
PORT="${PORT:-3000}"

usage() {
    cat <<EOF
Usage: $0 <backend> [options]

Backends:
    local          Use local JSON file storage (default)
    es, elasticsearch   Use Elasticsearch backend
    loki           Use Loki backend

Options:
    --port <port> Server port (default: 3000)
    --no-start    Don't start the server, just show the command

Examples:
    $0 local          Switch to local storage
    $0 es             Switch to Elasticsearch
    $0 loki --port 3001  Switch to Loki on port 3001

Environment Variables:
    LOG_STORAGE_BACKEND   Backend type (local|elasticsearch|loki)
    LOG_RETENTION_DAYS     Days to retain logs (default: 30)
    ELASTICSEARCH_URL      ES server URL
    LOKI_URL              Loki server URL
EOF
}

stop_server() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo "Stopping server (PID: $pid)..."
            kill "$pid" 2>/dev/null || true
            sleep 1
        fi
        rm -f "$PID_FILE"
    fi

    # Also try to kill by port
    local pids=$(lsof -ti ":$PORT" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        echo "Killing processes on port $PORT..."
        echo "$pids" | xargs kill 2>/dev/null || true
    fi
}

start_server() {
    local backend="$1"
    local port="${2:-$PORT}"

    echo "Starting server with backend: $backend on port: $port"

    export LOG_STORAGE_BACKEND="$backend"
    export PORT="$port"

    cd "$PROJECT_DIR"
    node server.js &
    local pid=$!

    echo "$pid" > "$PID_FILE"
    echo "Server started (PID: $pid)"

    # Wait for server to be ready
    for i in $(seq 1 30); do
        if curl -sf "http://localhost:$port/health" > /dev/null 2>&1; then
            echo "Server is ready at http://localhost:$port"
            echo ""
            echo "Backend info:"
            curl -sf "http://localhost:$port/api/logs/backend" | jq '.health'
            return 0
        fi
        sleep 1
    done

    echo "Error: Server did not start in time"
    return 1
}

main() {
    if [ $# -lt 1 ]; then
        usage
        exit 1
    fi

    local backend=""
    local port="$PORT"
    local no_start=false

    while [ $# -gt 0 ]; do
        case "$1" in
            local)
                backend="local"
                shift
                ;;
            es|elasticsearch)
                backend="elasticsearch"
                shift
                ;;
            loki)
                backend="loki"
                shift
                ;;
            --port)
                port="$2"
                shift 2
                ;;
            --no-start)
                no_start=true
                shift
                ;;
            -h|--help|help)
                usage
                exit 0
                ;;
            *)
                echo "Error: Unknown argument '$1'"
                usage
                exit 1
                ;;
        esac
    done

    if [ -z "$backend" ]; then
        echo "Error: Backend not specified"
        usage
        exit 1
    fi

    echo "=========================================="
    echo " DevOps Toolkit - Backend Switcher"
    echo "=========================================="
    echo ""

    if [ "$no_start" = "false" ]; then
        stop_server
        echo ""
        start_server "$backend" "$port"
    else
        echo "Command to start server with backend '$backend':"
        echo ""
        echo "    LOG_STORAGE_BACKEND=$backend PORT=$port node $SERVER_FILE"
        echo ""
    fi
}

main "$@"