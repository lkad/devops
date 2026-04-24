#!/bin/bash
# Test Physical Hosts via Docker SSH simulation
# This script starts Docker containers that act as SSH-enabled physical hosts

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
DATA_DIR="$SCRIPT_DIR/host-data"

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

# Start containers
start_hosts() {
    log_info "Creating data directories..."
    mkdir -p "$DATA_DIR"/host{1,2,3}

    log_info "Starting SSH-enabled containers..."
    cd "$SCRIPT_DIR"
    docker-compose up -d

    log_info "Waiting for SSH servers to be ready..."
    sleep 10

    # Check which containers are running
    docker ps --filter "name=physical-host" --format "{{.Names}} {{.Status}}"
}

# Stop containers
stop_hosts() {
    log_info "Stopping containers..."
    cd "$SCRIPT_DIR"
    docker-compose down 2>/dev/null || true
    log_success "Containers stopped"
}

# Show host status
show_status() {
    echo ""
    echo "=========================================="
    echo "       Physical Hosts Status"
    echo "=========================================="
    echo ""

    for i in 1 2 3; do
        CONTAINER="physical-host-$i"
        if docker ps --filter "name=$CONTAINER" --filter "status=running" | grep -q "$CONTAINER"; then
            echo -e "${GREEN}●${NC} $CONTAINER - Running"
            docker exec "$CONTAINER" cat /etc/hostname 2>/dev/null || true
        else
            echo -e "${RED}○${NC} $CONTAINER - Stopped"
        fi
    done
    echo ""
}

# Register hosts to system
register_hosts() {
    log_info "Creating host registration script..."

    cat > "$SCRIPT_DIR/register_and_test.js" << 'EOF'
const PhysicalHostManager = require('../k8s/physical_host_manager');

async function main() {
    const manager = new PhysicalHostManager({
        poolSize: 5,
        heartbeatInterval: 5000,
        commandTimeout: 10000
    });

    const hosts = [
        { id: 'host-1', hostname: 'prod-web-server-01', ip: 'localhost', port: 2221, username: 'root', authMethod: 'password', credentials: { password: 'test123' } },
        { id: 'host-2', hostname: 'prod-db-server-01', ip: 'localhost', port: 2222, username: 'root', authMethod: 'password', credentials: { password: 'test123' } },
        { id: 'host-3', hostname: 'prod-cache-server-01', ip: 'localhost', port: 2223, username: 'root', authMethod: 'password', credentials: { password: 'test123' } }
    ];

    console.log('\n==========================================');
    console.log('   Registering Physical Hosts');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            const host = manager.registerHost(hostInfo);
            console.log(`✓ Registered: ${host.hostname} (${host.ip}:${host.port})`);
        } catch (err) {
            console.log(`✗ Failed to register ${hostInfo.hostname}: ${err.message}`);
        }
    }

    console.log('\n==========================================');
    console.log('   Testing SSH Connections');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            console.log(`Testing connection to ${hostInfo.hostname}...`);
            await manager.getConnection(hostInfo.id);
            console.log(`✓ SSH connected to ${hostInfo.hostname}`);
        } catch (err) {
            console.log(`✗ SSH failed for ${hostInfo.hostname}: ${err.message}`);
        }
    }

    console.log('\n==========================================');
    console.log('   Collecting Metrics');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            console.log(`\n--- ${hostInfo.hostname} ---`);
            const metrics = await manager.collectMetrics(hostInfo.id);

            if (metrics.error) {
                console.log(`  Error: ${metrics.error}`);
            } else {
                console.log(`  CPU:    ${metrics.cpu?.usage ?? 'N/A'}% (${metrics.cpu?.cores ?? 0} cores)`);
                console.log(`  Memory: ${metrics.memory?.usagePercent ?? 0}% (${metrics.memory?.total ?? 0}MB total)`);
                if (metrics.disk?.disks) {
                    metrics.disk.disks.forEach(d => {
                        console.log(`  Disk:   ${d.device} ${d.percent}% used (${d.size})`);
                    });
                }
                console.log(`  Uptime: ${metrics.uptime?.formatted ?? 'N/A'}`);
                console.log(`  Collected at: ${metrics.collectedAt}`);
            }
        } catch (err) {
            console.log(`✗ Metrics failed for ${hostInfo.hostname}: ${err.message}`);
        }
    }

    console.log('\n==========================================');
    console.log('   Service Status');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            const service = await manager.checkService(hostInfo.id, 'ssh');
            console.log(`${hostInfo.hostname} - ssh: ${service.status}`);
        } catch (err) {
            console.log(`${hostInfo.hostname} - ssh: unknown (${err.message})`);
        }
    }

    console.log('\n==========================================');
    console.log('   Host Summary');
    console.log('==========================================\n');

    const summary = manager.getSummary();
    console.log(`Total hosts: ${summary.total}`);
    console.log(`Online: ${summary.byState.online}`);
    console.log(`Offline: ${summary.byState.offline}`);
    console.log(`Connection pool size: ${summary.poolSize}`);

    console.log('\n==========================================');
    console.log('   Get All Hosts Details');
    console.log('==========================================\n');

    const allHosts = manager.getAllHosts();
    allHosts.forEach(h => {
        console.log(`[${h.id}] ${h.hostname}`);
        console.log(`  IP: ${h.ip}:${h.port}, State: ${h.state}`);
        console.log(`  Last heartbeat: ${h.lastHeartbeat || 'Never'}`);
        if (h.metrics?.cpu) {
            console.log(`  CPU: ${h.metrics.cpu.usage}%, Memory: ${h.metrics.memory?.usagePercent}%`);
        }
        console.log('');
    });

    manager.shutdown();
    console.log('\nTest completed.');
}

main().catch(console.error);
EOF

    log_info "Running registration and metrics test..."
    cd /mnt/devops/devops-toolkit
    node test-physical-hosts/register_and_test.js
}

# Show usage
usage() {
    echo "Usage: $0 {start|stop|status|register|test|cleanup}"
    echo ""
    echo "Commands:"
    echo "  start    - Start SSH-enabled Docker containers"
    echo "  stop     - Stop all containers"
    echo "  status   - Show container status"
    echo "  register - Register hosts and collect metrics"
    echo "  test     - Full test (start + register)"
    echo "  cleanup  - Stop and remove all data"
}

# Main
case "${1:-help}" in
    start)
        check_docker
        start_hosts
        show_status
        ;;
    stop)
        stop_hosts
        ;;
    status)
        show_status
        ;;
    register)
        register_hosts
        ;;
    test)
        check_docker
        start_hosts
        sleep 3
        register_hosts
        ;;
    cleanup)
        stop_hosts
        rm -rf "$DATA_DIR"
        log_success "Cleaned up"
        ;;
    *)
        usage
        ;;
esac
