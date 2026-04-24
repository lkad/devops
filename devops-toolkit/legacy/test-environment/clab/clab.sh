#!/bin/bash
# Containerlab Dual-Datacenter HA Environment Manager
# ====================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAB_TOPOLOGY="$SCRIPT_DIR/topology.yml"

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

# Check if running as root (needed for clab)
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script needs root privileges"
        echo "Usage: sudo $0 $1"
        exit 1
    fi
}

# Check containerlab installation
check_clab() {
    if ! command -v clab &> /dev/null; then
        log_error "Containerlab not installed"
        echo "Run: sudo ./install.sh"
        exit 1
    fi
    clab version | head -1
}

# Deploy topology
deploy() {
    check_root "deploy"
    log_info "Deploying Containerlab topology..."
    check_clab

    cd "$SCRIPT_DIR"

    # Deploy
    clab deploy -t "$CLAB_TOPOLOGY"

    log_success "Containerlab deployed!"
    show_status
}

# Destroy topology
destroy() {
    check_root "destroy"
    log_info "Destroying Containerlab topology..."
    check_clab

    cd "$SCRIPT_DIR"
    clab destroy -t "$CLAB_TOPOLOGY" --cleanup

    log_success "Containerlab destroyed!"
}

# Show status
show_status() {
    echo ""
    echo "=========================================="
    echo "       Containerlab Topology Status"
    echo "=========================================="
    echo ""

    check_clab

    clab inspect -t "$CLAB_TOPOLOGY" 2>/dev/null || true

    echo ""
    echo "=========================================="
    echo "           Node Access"
    echo "=========================================="
    echo ""
    echo "DC1 (Left):"
    echo "  - dc1-sw1 (SNMP):  docker exec clab-devops-dc-ha-dc1-sw1 snmpwalk -v 2c -c public localhost"
    echo "  - dc1-web (SSH):   docker exec -it clab-devops-dc-ha-dc1-web bash"
    echo "  - dc1-db (SSH):    docker exec -it clab-devops-dc-ha-dc1-db bash"
    echo ""
    echo "DC2 (Right):"
    echo "  - dc2-sw1 (SNMP):  docker exec clab-devops-dc-ha-dc2-sw1 snmpwalk -v 2c -c public localhost"
    echo "  - dc2-web (SSH):   docker exec -it clab-devops-dc-ha-dc2-web bash"
    echo "  - dc2-db (SSH):    docker exec -it clab-devops-dc-ha-dc2-db bash"
    echo ""
}

# Test connectivity
test_connectivity() {
    log_info "Testing connectivity..."

    local errors=0

    # Test SSH to DC1 web
    if docker exec clab-devops-dc-ha-dc1-web hostname &>/dev/null; then
        log_success "DC1 Web SSH - Connected ($(docker exec clab-devops-dc-ha-dc1-web hostname 2>/dev/null))"
    else
        log_error "DC1 Web SSH - Failed"
        errors=$((errors + 1))
    fi

    # Test SSH to DC2 web
    if docker exec clab-devops-dc-ha-dc2-web hostname &>/dev/null; then
        log_success "DC2 Web SSH - Connected ($(docker exec clab-devops-dc-ha-dc2-web hostname 2>/dev/null))"
    else
        log_error "DC2 Web SSH - Failed"
        errors=$((errors + 1))
    fi

    # Test SSH to DC1 db
    if docker exec clab-devops-dc-ha-dc1-db hostname &>/dev/null; then
        log_success "DC1 DB SSH - Connected ($(docker exec clab-devops-dc-ha-dc1-db hostname 2>/dev/null))"
    else
        log_error "DC1 DB SSH - Failed"
        errors=$((errors + 1))
    fi

    # Test SNMP DC1
    if docker exec clab-devops-dc-ha-dc1-sw1 snmpstatus -v 2c -c public localhost &>/dev/null; then
        log_success "DC1 Switch SNMP - Connected"
    else
        log_error "DC1 Switch SNMP - Failed"
        errors=$((errors + 1))
    fi

    # Test SNMP DC2
    if docker exec clab-devops-dc-ha-dc2-sw1 snmpstatus -v 2c -c public localhost &>/dev/null; then
        log_success "DC2 Switch SNMP - Connected"
    else
        log_error "DC2 Switch SNMP - Failed"
        errors=$((errors + 1))
    fi

    # Test inter-DC link
    if docker exec clab-devops-dc-ha-dc1-web ping -c 1 -W 2 172.30.30.31 &>/dev/null; then
        log_success "Inter-DC Link (DC1 -> DC2 core) - Connected"
    else
        log_warn "Inter-DC Link (DC1 -> DC2 core) - Failed"
    fi

    echo ""
    if [ $errors -eq 0 ]; then
        log_success "All connectivity tests passed!"
    else
        log_error "$errors test(s) failed"
    fi

    return $errors
}

# Generate graph (拓扑图)
graph() {
    check_root "graph"
    check_clab
    clab graph -t "$CLAB_TOPOLOGY" -o "$SCRIPT_DIR/topology.svg"
    log_success "Topology graph saved to $SCRIPT_DIR/topology.svg"
}

# Show logs
logs() {
    local node=${2:-""}
    if [ -z "$node" ]; then
        echo "Usage: $0 logs <node-name>"
        echo "Nodes: dc1-sw1, dc1-sw2, dc1-web, dc1-db, dc2-sw1, dc2-sw2, dc2-web, dc2-db"
        exit 1
    fi
    docker logs "clab-devops-dc-ha-$node" --tail 100 -f
}

# Usage
usage() {
    echo "Containerlab Dual-Datacenter HA Manager"
    echo ""
    echo "Usage: sudo $0 {deploy|estroy|status|test|graph|logs|help}"
    echo ""
    echo "Commands:"
    echo "  deploy  - Deploy the containerlab topology"
    echo "  destroy - Destroy the containerlab topology"
    echo "  status  - Show topology status"
    echo "  test    - Test connectivity to all nodes"
    echo "  graph   - Generate topology graph (SVG)"
    echo "  logs    - Show logs for a node (Usage: logs <node-name>)"
    echo "  help    - Show this help"
    echo ""
    echo "Examples:"
    echo "  sudo $0 deploy"
    echo "  sudo $0 status"
    echo "  sudo $0 test"
    echo "  sudo $0 logs dc1-sw1"
    echo "  sudo $0 destroy"
}

# Main
case "${1:-help}" in
    deploy)
        deploy
        ;;
    destroy)
        destroy
        ;;
    status)
        show_status
        ;;
    test)
        test_connectivity
        ;;
    graph)
        graph
        ;;
    logs)
        logs "$@"
        ;;
    help|*)
        usage
        ;;
esac
