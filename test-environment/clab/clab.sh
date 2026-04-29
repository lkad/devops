#!/bin/bash
# Containerlab 双数据中心拓扑管理脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOPOLOGY_FILE="${SCRIPT_DIR}/topology.yml"
LAB_NAME="devops-toolkit-test"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 containerlab 是否安装
check_containerlab() {
    if ! command -v containerlab &> /dev/null; then
        log_error "Containerlab 未安装。请运行: ./install.sh"
        exit 1
    fi
}

# 部署所有集群
deploy() {
    check_containerlab
    log_info "部署 Containerlab 拓扑..."
    sudo containerlab deploy -t ${TOPOLOGY_FILE}
    log_info "部署完成!"
}

# 销毁所有集群
destroy() {
    check_containerlab
    log_info "销毁 Containerlab 拓扑..."
    sudo containerlab destroy -t ${TOPOLOGY_FILE} --cleanup
    log_info "销毁完成!"
}

# 显示状态
status() {
    check_containerlab
    log_info "Containerlab 拓扑状态:"
    sudo containerlab inspect -t ${TOPOLOGY_FILE}
}

# 启动已部署的拓扑
start() {
    check_containerlab
    log_info "启动 Containerlab 拓扑..."
    sudo containerlab start -t ${TOPOLOGY_FILE}
    log_info "启动完成!"
}

# 停止拓扑
stop() {
    check_containerlab
    log_info "停止 Containerlab 拓扑..."
    sudo containerlab stop -t ${TOPOLOGY_FILE}
    log_info "停止完成!"
}

# 测试连接
test() {
    log_info "测试连接性..."

    # 测试 DC1 设备
    log_info "测试 DC1 设备..."
    for host in 172.30.30.11 172.30.30.12 172.30.30.21 172.30.30.22; do
        if ping -c 1 -W 1 $host &> /dev/null; then
            log_info "  $host - OK"
        else
            log_error "  $host - FAILED"
        fi
    done

    # 测试 DC2 设备
    log_info "测试 DC2 设备..."
    for host in 172.30.30.31 172.30.30.32 172.30.30.41 172.30.30.42; do
        if ping -c 1 -W 1 $host &> /dev/null; then
            log_info "  $host - OK"
        else
            log_error "  $host - FAILED"
        fi
    done

    # 测试 SSH 连接
    log_info "测试 SSH 连接..."
    for port in 2221 2222 4221 4222; do
        if nc -z -w 1 localhost $port 2>/dev/null; then
            log_info "  localhost:$port - OK"
        else
            log_warn "  localhost:$port - FAILED (SSH 服务可能未启动)"
        fi
    done

    # 测试 SNMP 连接
    log_info "测试 SNMP 连接..."
    for host in 172.30.30.11 172.30.30.31; do
        if snmpwalk -v2c -c public $host 1.3.6.1.2.1.1.1.0 &> /dev/null; then
            log_info "  $host SNMP - OK"
        else
            log_warn "  $host SNMP - FAILED"
        fi
    done
}

# 列出所有节点
list() {
    check_containerlab
    echo ""
    echo "=== DevOps Toolkit Test Environment Nodes ==="
    echo ""
    echo "DC1 (Left Data Center):"
    echo "  dc1-sw1  - Core Switch     - 172.30.30.11 - SNMP (UDP:161)"
    echo "  dc1-sw2  - Distribution    - 172.30.30.12 - SNMP (UDP:161)"
    echo "  dc1-web  - Web Server      - 172.30.30.21 - SSH (TCP:2221)"
    echo "  dc1-db   - DB Server       - 172.30.30.22 - SSH (TCP:2222)"
    echo ""
    echo "DC2 (Right Data Center):"
    echo "  dc2-sw1  - Core Switch     - 172.30.30.31 - SNMP (UDP:361)"
    echo "  dc2-sw2  - Distribution    - 172.30.30.32 - SNMP (UDP:362)"
    echo "  dc2-web  - Web Server      - 172.30.30.41 - SSH (TCP:4221)"
    echo "  dc2-db   - DB Server       - 172.30.30.42 - SSH (TCP:4222)"
    echo ""
    echo "Time Series Databases:"
    echo "  InfluxDB  - 8086"
    echo "  Prometheus - 9090"
    echo "  Grafana   - 3001"
    echo ""
}

# 生成 kubeconfig (用于 K8s 测试)
generate-kubeconfig() {
    check_containerlab
    log_info "生成 kubeconfig..."
    sudo containerlab inspect -t ${TOPOLOGY_FILE} | grep -A 50 "kubeconfig" || true
}

# 显示帮助
show_help() {
    echo "DevOps Toolkit Containerlab 管理脚本"
    echo ""
    echo "用法: $0 <command>"
    echo ""
    echo "命令:"
    echo "  deploy    - 部署所有节点"
    echo "  destroy   - 销毁所有节点"
    echo "  start     - 启动已部署的拓扑"
    echo "  stop      - 停止拓扑"
    echo "  status    - 显示拓扑状态"
    echo "  test      - 测试连接性"
    echo "  list      - 列出所有节点"
    echo "  gen-kube  - 生成 kubeconfig"
    echo "  help      - 显示帮助"
    echo ""
    echo "示例:"
    echo "  $0 deploy    # 部署环境"
    echo "  $0 test      # 测试连接"
    echo "  $0 destroy   # 清理环境"
}

# 主入口
case "${1:-help}" in
    deploy)
        deploy
        ;;
    destroy)
        destroy
        ;;
    start)
        start
        ;;
    stop)
        stop
        ;;
    status)
        status
        ;;
    test)
        test
        ;;
    list)
        list
        ;;
    gen-kube)
        generate-kubeconfig
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        log_error "未知命令: $1"
        show_help
        exit 1
        ;;
esac
