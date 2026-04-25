#!/bin/bash
# Test runner script for DevOps Toolkit
# This script starts the required dependencies and runs tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== DevOps Toolkit Test Runner ==="

# Parse arguments
RUN_QA=false
VERBOSE=""
TEST_TARGET="./..."

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -qa|--qa)
            RUN_QA=true
            shift
            ;;
        -run)
            TEST_TARGET="$2"
            shift 2
            ;;
        *)
            TEST_TARGET="$1"
            shift
            ;;
    esac
done

# Function to cleanup
cleanup() {
    echo ""
    echo "Cleaning up..."
    # Kill server if we started it
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "Stopping devops-toolkit server (PID: $SERVER_PID)..."
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    cd "$PROJECT_DIR"
    docker-compose -f docker-compose.yml down 2>/dev/null || true
}

trap cleanup EXIT

# Check if docker is available
check_docker() {
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}Error: docker is not installed${NC}"
        exit 1
    fi

    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}Error: docker-compose is not installed${NC}"
        exit 1
    fi
}

# Start PostgreSQL
start_postgres() {
    # Check if PostgreSQL is already running
    if docker ps | grep -q devops-postgres; then
        echo -e "${GREEN}✓${NC} PostgreSQL is already running"
    else
        echo "Starting PostgreSQL..."
        docker run -d \
            --name devops-postgres \
            -e POSTGRES_USER=devops \
            -e POSTGRES_PASSWORD=devops \
            -e POSTGRES_DB=devops_toolkit \
            -p 5432:5432 \
            postgres:16-alpine

        # Wait for PostgreSQL to be ready
        echo "Waiting for PostgreSQL to be ready..."
        for i in {1..30}; do
            if docker exec devops-postgres pg_isready -U devops -d devops_toolkit &>/dev/null; then
                echo -e "${GREEN}✓${NC} PostgreSQL is ready!"
                break
            fi
            sleep 1
        done
    fi
}

# Start devops-toolkit server
start_server() {
    # Check if server is already running
    if curl -s http://localhost:3000/health &>/dev/null; then
        echo -e "${GREEN}✓${NC} DevOps Toolkit server is already running on port 3000"
        return
    fi

    echo "Starting DevOps Toolkit server..."
    cd "$PROJECT_DIR"
    go run ./cmd/devops-toolkit/main.go > /tmp/devops-server.log 2>&1 &
    SERVER_PID=$!

    # Wait for server to start
    echo "Waiting for server to start..."
    for i in {1..30}; do
        if curl -s http://localhost:3000/health &>/dev/null; then
            echo -e "${GREEN}✓${NC} DevOps Toolkit server is ready (PID: $SERVER_PID)"
            return
        fi
        # Check if process is still running
        if ! kill -0 "$SERVER_PID" 2>/dev/null; then
            echo -e "${RED}✗${NC} Server failed to start!"
            echo "Server log:"
            cat /tmp/devops-server.log
            exit 1
        fi
        sleep 1
    done

    echo -e "${RED}✗${NC} Server did not respond in time"
    exit 1
}

# Reset database
reset_database() {
    echo "Resetting database..."
    docker exec devops-postgres psql -U devops -d devops_toolkit -c 'DROP TABLE IF EXISTS project_permissions, project_resources, projects, systems, business_lines, devices CASCADE;' 2>/dev/null || true
    # Restart server to recreate tables
    echo "Restarting server to recreate tables..."
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    sleep 1
    go run ./cmd/devops-toolkit/main.go > /tmp/devops-server.log 2>&1 &
    SERVER_PID=$!
    for i in {1..30}; do
        if curl -s http://localhost:3000/health &>/dev/null; then
            echo -e "${GREEN}✓${NC} Server restarted"
            return
        fi
        sleep 1
    done
    echo -e "${RED}✗${NC} Server failed to restart"
    exit 1
}

# Run tests
run_tests() {
    echo ""
    echo "=== Running Tests ==="
    echo ""

    if [ "$VERBOSE" = "-v" ]; then
        go test $VERBOSE -count=1 $TEST_TARGET
    else
        go test -count=1 $TEST_TARGET
    fi

    echo ""
    echo "=== Test Run Complete ==="
}

# Main execution
check_docker

echo ""
echo "--- Starting Services ---"
start_postgres

if [ "$RUN_QA" = true ]; then
    start_server
    reset_database
else
    echo ""
    echo "--- QA Tests Note ---"
    echo -e "${YELLOW}Skipping QA tests (server not started).${NC}"
    echo "To run QA integration tests, use: $0 -qa"
    echo ""
fi

run_tests
