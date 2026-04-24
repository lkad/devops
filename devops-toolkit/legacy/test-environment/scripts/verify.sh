#!/bin/bash
# verify.sh - 验证测试环境中所有设备连接

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

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

ERRORS=0

echo "=========================================="
echo "   Test Environment Connection Verify"
echo "=========================================="
echo ""

# Check if docker is running
check_docker() {
    if ! docker info &> /dev/null; then
        log_error "Docker daemon not running"
        exit 1
    fi
}

# Verify SSH Connections
verify_ssh() {
    log_info "=== SSH Physical Hosts ==="

    local ssh_hosts=("physical-host-1:2221:prod-web-server-01" "physical-host-2:2222:prod-db-server-01" "physical-host-3:2223:prod-cache-server-01")

    for entry in "${ssh_hosts[@]}"; do
        IFS=':' read -r name port hostname <<< "$entry"

        if docker ps --filter "name=$name" --filter "status=running" | grep -q "$name"; then
            # Test SSH connection
            if timeout 5 ssh -o StrictHostKeyChecking=no -o ConnectTimeout=3 -p $port root@localhost "echo ok" &>/dev/null; then
                log_success "$name ($hostname:$port) - SSH connected"

                # Get hostname inside container
                container_hostname=$(docker exec $name hostname 2>/dev/null || echo "unknown")
                log_info "  Container hostname: $container_hostname"
            else
                log_error "$name ($hostname:$port) - SSH connection failed"
                ERRORS=$((ERRORS + 1))
            fi
        else
            log_warn "$name - Container not running"
            ERRORS=$((ERRORS + 1))
        fi
    done
    echo ""
}

# Verify SNMP Connections
verify_snmp() {
    log_info "=== SNMP Network Devices ==="

    local snmp_devices=("snmp-switch:8161:core-switch-01" "snmp-router:8162:edge-router-01" "snmp-firewall:8163:fw-gateway-01")

    for entry in "${snmp_devices[@]}"; do
        IFS=':' read -r name port hostname <<< "$entry"

        if docker ps --filter "name=$name" --filter "status=running" | grep -q "$name"; then
            # Test SNMP
            if timeout 5 snmpget -v 2c -c public -r 1 localhost:$port sysDescr.0 &>/dev/null; then
                log_success "$name ($hostname:$port) - SNMP connected"

                # Get sysDescr
                descr=$(snmpget -v 2c -c public localhost:$port sysDescr.0 2>/dev/null | awk '{print $NF}')
                log_info "  sysDescr: $descr"
            else
                log_warn "$name ($hostname:$port) - SNMP query failed (may need container network)"
                # Try from within network
                if docker exec devops-toolkit snmpget -v 2c -c public $name:$port sysDescr.0 &>/dev/null; then
                    log_success "$name - SNMP connected (via container)"
                else
                    log_error "$name - SNMP connection failed"
                    ERRORS=$((ERRORS + 1))
                fi
            fi
        else
            log_warn "$name - Container not running"
        fi
    done
    echo ""
}

# Verify HAProxy Connections
verify_haproxy() {
    log_info "=== HAProxy Load Balancers ==="

    local haproxy_instances=("haproxy-lb-1:8000:web-lb-01")

    for entry in "${haproxy_instances[@]}"; do
        IFS=':' read -r name port hostname <<< "$entry"

        if docker ps --filter "name=$name" --filter "status=running" | grep -q "$name"; then
            # Test HTTP
            if curl -s -o /dev/null -w "%{http_code}" http://localhost:$port/haproxy_stats &>/dev/null; then
                log_success "$name ($hostname:$port) - HTTP connected"
            else
                log_warn "$name - HTTP not responding"
                ERRORS=$((ERRORS + 1))
            fi
        else
            log_warn "$name - Container not running"
        fi
    done
    echo ""
}

# Verify Time Series Databases
verify_tsdb() {
    log_info "=== Time Series Databases ==="

    # InfluxDB
    if curl -s http://localhost:8086/health | grep -q "pass"; then
        log_success "InfluxDB (:8086) - Healthy"

        # Query buckets
        buckets=$(curl -s -H "Authorization: Token devops-token" http://localhost:8086/api/v2/buckets 2>/dev/null | grep -o '"name":"[^"]*"' | head -3 || echo "query failed")
        log_info "  Buckets: $buckets"
    else
        log_error "InfluxDB (:8086) - Not responding"
        ERRORS=$((ERRORS + 1))
    fi

    # Prometheus
    if curl -s http://localhost:9090/-/healthy | grep -q "Prometheus"; then
        log_success "Prometheus (:9090) - Healthy"

        # Count targets
        targets=$(curl -s http://localhost:9090/api/v1/targets | grep -o '"health":"up"' | wc -l)
        log_info "  Active targets: $targets"
    else
        log_error "Prometheus (:9090) - Not responding"
        ERRORS=$((ERRORS + 1))
    fi

    # Grafana
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:3001 | grep -q "302"; then
        log_success "Grafana (:3001) - Healthy"
    else
        log_error "Grafana (:3001) - Not responding"
        ERRORS=$((ERRORS + 1))
    fi

    echo ""
}

# Verify Container Network
verify_network() {
    log_info "=== Container Network ==="

    # Check if containers are on same network
    physical_hosts=$(docker network inspect test-environment_physical-net --format '{{range .Containers}}{{.Name}} {{end}}' 2>/dev/null | tr ' ' '\n' | grep -c "physical-host" || echo "0")
    if [ "$physical_hosts" -ge 3 ]; then
        log_success "Physical hosts network - Connected ($physical_hosts containers)"
    else
        log_warn "Physical hosts network - Some containers may be isolated"
    fi

    echo ""
}

# Summary
show_summary() {
    echo "=========================================="
    echo "           Verification Summary"
    echo "=========================================="

    if [ $ERRORS -eq 0 ]; then
        log_success "All checks passed!"
        return 0
    else
        log_error "$ERRORS check(s) failed"
        return 1
    fi
}

# Main
check_docker
verify_ssh
verify_snmp
verify_haproxy
verify_tsdb
verify_network
show_summary
