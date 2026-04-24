# Containerlab 双数据中心高可用拓扑

## 拓扑架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Containerlab Network                          │
│                     172.30.30.0/24                              │
│                                                                  │
│  ┌──────────────┐                        ┌──────────────┐        │
│  │   DC1 (Left) │                        │  DC2 (Right) │        │
│  │  10.0.1.0/24 │                        │  10.0.2.0/24 │        │
│  │              │                        │              │        │
│  │ dc1-sw1 ────╫═══════════════════════╫── dc2-sw1   │        │
│  │  (Core)     ╫══ Dual Trunk (eth2,eth3)══  (Core)  │        │
│  │      │      ╫═══════════════════════╫══      │    │        │
│  │ dc1-sw2     ╫═══════════════════════╫     dc2-sw2 │        │
│  │  (Dist)     ╫═══════════════════════╫   (Dist)    │        │
│  │   │    │    ╫═══════════════════════╫    │    │    │        │
│  │   │    │    ╫═══════════════════════╫    │    │    │        │
│  │ ┌─┴┐ ┌─┴┐ ╫═══════════════════════╫ ┌──┴┐ ┌──┴┐ │        │
│  │ │W │ │D │ ╫═══════════════════════╫ │W  │ │D  │ │        │
│  │ │eb│ │b  │                            │eb │ │b  │ │        │
│  │ │1 │ │1  │                            │2  │ │2  │ │        │
│  │ └──┘ └──┘                            └──┘ └──┘ │        │
│  └──────────────┘                        └──────────────┘        │
└─────────────────────────────────────────────────────────────────┘

W = Web Server (Ubuntu + SSH)
D = DB Server (Ubuntu + SSH)
sw = Switch (Ubuntu + SNMP)
```

## 节点清单

| 节点 | 角色 | MGMT IP | 服务 |
|------|------|---------|------|
| dc1-sw1 | DC1 Core Switch | 172.30.30.11 | SNMP (UDP:161) |
| dc1-sw2 | DC1 Distribution | 172.30.30.12 | SNMP (UDP:161) |
| dc1-web | DC1 Web Server | 172.30.30.21 | SSH |
| dc1-db | DC1 DB Server | 172.30.30.22 | SSH |
| dc2-sw1 | DC2 Core Switch | 172.30.30.31 | SNMP (UDP:161) |
| dc2-sw2 | DC2 Distribution | 172.30.30.32 | SNMP (UDP:161) |
| dc2-web | DC2 Web Server | 172.30.30.41 | SSH |
| dc2-db | DC2 DB Server | 172.30.30.42 | SSH |

## 高可用特性

1. **双数据中心** - DC1 和 DC2 独立部署
2. **双Trunk链路** - dc1-sw1 和 dc2-sw1 之间有两条 trunk 链路 (eth2, eth3)
3. **交换架构** - 每个DC有 core switch + distribution switch
4. **SSH服务** - 每台服务器运行真实 SSH daemon

## 安装 Containerlab

```bash
# 安装 containerlab
curl -sL https://get.containerlab.dev | sudo bash

# 添加用户到 clab_admins 组
sudo usermod -aG clab_admins $USER
newgrp clab_admins
```

## 部署命令

```bash
cd /mnt/devops/devops-toolkit/test-environment/clab

# 部署拓扑
sudo clab deploy -t topology.yml

# 查看状态
sudo clab inspect -t topology.yml

# 测试连通性
./clab.sh test

# 销毁拓扑
sudo clab destroy -t topology.yml --cleanup
```

## 测试连接

```bash
# 测试 SSH 到 DC1 Web
docker exec clab-devops-dc-ha-dc1-web hostname

# 测试 SSH 到 DC2 DB
docker exec clab-devops-dc-ha-dc2-db hostname

# 测试 SNMP DC1 Switch
docker exec clab-devops-dc-ha-dc1-sw1 snmpwalk -v 2c -c public localhost

# 测试 SNMP DC2 Switch
docker exec clab-devops-dc-ha-dc2-sw1 snmpstatus -v 2c -c public localhost

# 从外部测试 SNMP (需要暴露端口)
# dc1-sw1 SNMP 监听在 172.30.30.11:161
```

## 资源需求

- CPU: 4+ cores
- Memory: 4GB+
- Disk: 10GB+
- Docker: 20GB+ (用于镜像存储)

## 故障排查

```bash
# 查看容器日志
docker logs clab-devops-dc-ha-dc1-web
docker logs clab-devops-dc-ha-dc1-sw1

# 进入容器调试
docker exec -it clab-devops-dc-ha-dc1-web bash

# 重启单个节点
sudo clab deploy -t topology.yml --reconfigure

# 检查网络连接
docker exec clab-devops-dc-ha-dc1-web ping -c 3 172.30.30.31
```
