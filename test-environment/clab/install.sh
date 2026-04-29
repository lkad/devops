#!/bin/bash
# Containerlab 安装脚本

set -e

echo "安装 Containerlab..."

# 检查系统要求
if [ "$EUID" -ne 0 ]; then
    echo "请使用 sudo 运行此脚本"
    exit 1
fi

# 安装依赖
echo "安装系统依赖..."
apt-get update
apt-get install -y \
    curl \
    iproute2 \
    iputils-ping \
    net-tools \
    openssh-client \
    socat \
    graphviz \
    docker.io \
    docker-compose

# 启用并启动 Docker
systemctl enable docker
systemctl start docker

# 添加当前用户到 docker 组
if [ -n "$SUDO_USER" ]; then
    usermod -aG docker $SUDO_USER
fi

# 安装 Containerlab
echo "安装 Containerlab..."
curl -sL https://containerlab.dev/install.sh | bash

# 验证安装
containerlab version

echo ""
echo "Containerlab 安装完成!"
echo ""
echo "下一步:"
echo "  1. 运行 ./clab.sh deploy 部署测试环境"
echo "  2. 运行 ./clab.sh test 测试连接"
