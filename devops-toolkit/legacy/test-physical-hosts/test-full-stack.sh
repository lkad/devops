#!/bin/bash
# Test Physical Hosts - Full Stack
# 启动完整的测试环境: SSH containers + InfluxDB + Prometheus + Grafana

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check docker
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker not found"
        exit 1
    fi
    if ! docker info &> /dev/null; then
        log_error "Docker daemon not running"
        exit 1
    fi
}

# Start all containers
start_all() {
    log_info "Starting all containers..."
    cd "$SCRIPT_DIR"
    docker-compose up -d

    log_info "Waiting for services to be ready..."
    sleep 15

    # Check service health
    log_info "Checking service health..."
    docker ps --filter "name=physical-host" --format "{{.Names}} {{.Status}}"
    docker ps --filter "name=devops-" --format "{{.Names}} {{.Status}}"
    docker ps --filter "name=mock-" --format "{{.Names}} {{.Status}}"

    echo ""
    log_success "Services started:"
    echo "  - SSH containers: 2221, 2222, 2223"
    echo "  - InfluxDB: http://localhost:8086"
    echo "  - Prometheus: http://localhost:9090"
    echo "  - Grafana: http://localhost:3001 (admin/admin)"
    echo "  - Mock Telegraf Agent: http://localhost:8090"
}

# Stop all containers
stop_all() {
    log_info "Stopping all containers..."
    cd "$SCRIPT_DIR"
    docker-compose down 2>/dev/null || true
    log_success "All containers stopped"
}

# Show status
show_status() {
    echo ""
    echo "=========================================="
    echo "       Physical Hosts Full Stack Status"
    echo "=========================================="
    echo ""

    log_info "SSH Containers:"
    for i in 1 2 3; do
        CONTAINER="physical-host-$i"
        if docker ps --filter "name=$CONTAINER" --filter "status=running" | grep -q "$CONTAINER"; then
            echo -e "  ${GREEN}●${NC} $CONTAINER - Running"
        else
            echo -e "  ${RED}○${NC} $CONTAINER - Stopped"
        fi
    done

    log_info "Time Series Databases:"
    for container in devops-influxdb devops-prometheus devops-grafana; do
        if docker ps --filter "name=$container" --filter "status=running" | grep -q "$container"; then
            echo -e "  ${GREEN}●${NC} $container - Running"
        else
            echo -e "  ${RED}○${NC} $container - Stopped"
        fi
    done

    log_info "Mock Agent:"
    if docker ps --filter "name=mock-telegraf-agent" --filter "status=running" | grep -q "mock-telegraf-agent"; then
        echo -e "  ${GREEN}●${NC} mock-telegraf-agent - Running"
    else
        echo -e "  ${RED}○${NC} mock-telegraf-agent - Stopped"
    fi

    echo ""
}

# Verify stack
verify_stack() {
    log_info "Verifying stack..."

    local errors=0

    # Check InfluxDB
    if curl -s http://localhost:8086/health | grep -q "{\"status\":\"pass\""; then
        log_success "InfluxDB is healthy"
    else
        log_error "InfluxDB is not responding"
        errors=$((errors + 1))
    fi

    # Check Prometheus
    if curl -s http://localhost:9090/-/healthy | grep -q "Prometheus"; then
        log_success "Prometheus is healthy"
    else
        log_error "Prometheus is not responding"
        errors=$((errors + 1))
    fi

    # Check Mock Agent metrics
    if curl -s http://localhost:8090/metrics | grep -q "cpu_usage"; then
        log_success "Mock Agent is exposing metrics"
    else
        log_error "Mock Agent metrics endpoint not working"
        errors=$((errors + 1))
    fi

    if [ $errors -eq 0 ]; then
        log_success "All services verified!"
    else
        log_warn "$errors services may have issues"
    fi

    return $errors
}

# Show usage
usage() {
    echo "Usage: $0 {start|stop|status|verify|restart|cleanup}"
    echo ""
    echo "Commands:"
    echo "  start    - Start all containers (SSH + DBs + Mock Agent)"
    echo "  stop     - Stop all containers"
    echo "  restart  - Restart all containers"
    echo "  status   - Show container status"
    echo "  verify   - Verify all services are healthy"
    echo "  cleanup  - Stop and remove all containers and volumes"
}

# Main
case "${1:-help}" in
    start)
        check_docker
        start_all
        show_status
        ;;
    stop)
        stop_all
        ;;
    restart)
        check_docker
        stop_all
        sleep 2
        start_all
        show_status
        ;;
    status)
        show_status
        ;;
    verify)
        verify_stack
        ;;
    cleanup)
        stop_all
        log_info "Removing volumes..."
        cd "$SCRIPT_DIR"
        docker-compose down -v 2>/dev/null || true
        log_success "Cleaned up"
        ;;
    *)
        usage
        ;;
esac
