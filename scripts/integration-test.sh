#!/bin/bash
# DevOps Toolkit 集成测试脚本
# 需要先启动 docker-compose 和 containerlab 环境

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查环境
check_env() {
    log_info "检查测试环境..."

    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi

    # 检查 Go
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装"
        exit 1
    fi

    # 检查 DevOps Toolkit 服务
    if ! curl -s http://localhost:3000/health &> /dev/null; then
        log_warn "DevOps Toolkit 服务未运行在 localhost:3000"
        log_info "请先启动服务: go run cmd/devops-toolkit/main.go"
    fi

    log_info "环境检查完成"
}

# 启动测试环境
start_env() {
    log_info "启动测试环境..."

    # 启动 Docker Compose 服务
    cd "${SCRIPT_DIR}/../test-environment"
    docker-compose up -d
    cd "${PROJECT_ROOT}"

    log_info "等待服务启动..."
    sleep 10

    # 检查服务健康
    for service in influxdb prometheus grafana loki; do
        if docker ps | grep -q "devops-$service"; then
            log_info "  $service - OK"
        else
            log_warn "  $service - 未运行"
        fi
    done
}

# 运行集成测试
run_integration_tests() {
    log_info "运行集成测试..."

    export DEVOPS_TEST_URL=http://localhost:3000

    # 运行项目管理的集成测试
    go test -v -tags=integration ./internal/project/... 2>&1 || true

    # 运行设备管理的集成测试
    go test -v -tags=integration ./internal/device/... 2>&1 || true
}

# 运行所有测试
run_all_tests() {
    log_info "运行所有测试..."

    # 运行单元测试
    log_info "运行单元测试..."
    go test ./internal/auth/... ./internal/rbac/... 2>&1

    # 运行集成测试
    log_info "运行集成测试..."
    run_integration_tests
}

# 停止测试环境
stop_env() {
    log_info "停止测试环境..."

    cd "${SCRIPT_DIR}/../test-environment"
    docker-compose down
    cd "${PROJECT_ROOT}"

    log_info "测试环境已停止"
}

# 显示帮助
show_help() {
    echo "DevOps Toolkit 集成测试脚本"
    echo ""
    echo "用法: $0 <command>"
    echo ""
    echo "命令:"
    echo "  check   - 检查测试环境"
    echo "  start   - 启动测试环境 (Docker Compose + Containerlab)"
    echo "  test    - 运行集成测试"
    echo "  all     - 运行所有测试 (单元 + 集成)"
    echo "  stop    - 停止测试环境"
    echo "  help    - 显示帮助"
    echo ""
    echo "前置条件:"
    echo "  1. Docker 和 Docker Compose 已安装"
    echo "  2. Containerlab 已安装 (用于网络模拟)"
    echo "  3. DevOps Toolkit 服务运行在 localhost:3000"
    echo ""
    echo "示例:"
    echo "  $0 check    # 检查环境"
    echo "  $0 start    # 启动测试环境"
    echo "  $0 test     # 运行测试"
    echo "  $0 stop     # 停止环境"
}

# 主入口
case "${1:-help}" in
    check)
        check_env
        ;;
    start)
        start_env
        ;;
    test)
        run_integration_tests
        ;;
    all)
        run_all_tests
        ;;
    stop)
        stop_env
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
