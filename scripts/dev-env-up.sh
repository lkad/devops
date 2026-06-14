#!/usr/bin/env bash
# scripts/dev-env-up.sh — start the full dev environment: 8-service
# docker-compose stack + 8-node containerlab + register all 8 hosts
# as physical_host devices for the monitor loop to probe.
#
# Pair with dev-env-down.sh to teardown.
#
# Flow:
#   1. Destroy any leftover containerlab state from a prior session
#   2. Bring up the 8-service compose stack (postgres, loki, es,
#      prometheus, grafana, alertmanager, ldap, redis, devops-toolkit)
#   3. Wait for devops-toolkit's /api/v1/capabilities to return 200
#      (LDAP, postgres, redis are deps; AutoMigrate is the slow bit)
#   4. Bring up the 8-node containerlab topology
#   5. Read IPs from topology-data.json, register each physical host
#      via POST /api/v1/devices + /api/v1/physical-hosts
#   6. Trigger one manual probe on each host so the monitor loop has
#      data on the next 1m tick (otherwise the first real tick is 60s away)
#   7. Print access URLs and the SSH prober key fingerprint
#
# Required on the host:
#   - docker + docker compose
#   - containerlab (clab)
#   - jq, curl, ssh-keygen
#
# Optional: prober private key at deploy/secrets/prober/id_ed25519 —
# if missing, the script generates one and re-templated PUBLIC_KEY in
# the topology will be substituted at deploy time.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"
log_init "$(basename "${BASH_SOURCE[0]}")"

DEPLOY_DIR="$REPO_ROOT/deploy"
TOPOLOGY="$DEPLOY_DIR/containerlab/topology.yml"
PROBER_KEY_DIR="$DEPLOY_DIR/secrets/prober"
PROBER_KEY="$PROBER_KEY_DIR/id_ed25519"
API_BASE="${API_BASE:-http://localhost:3000/api/v1}"
API_USER="${API_USER:-alice}"

# 1. 销毁旧 clab 残留
if [ -d "$DEPLOY_DIR/containerlab/clab-devops-toolkit-clab" ]; then
	log_step "Destroying leftover containerlab state from prior session"
	(cd "$DEPLOY_DIR" && containerlab destroy -t containerlab/topology.yml --cleanup) \
		|| log_warn "containerlab destroy failed (continuing)"
fi

# 2. (re)generate prober keypair if missing
mkdir -p "$PROBER_KEY_DIR"
if [ ! -f "$PROBER_KEY" ]; then
	log_step "Generating ed25519 prober keypair at $PROBER_KEY"
	ssh-keygen -t ed25519 -f "$PROBER_KEY" -N "" -C "prober@devops-toolkit" -q
	log_ok "Key generated; private key is gitignored"
fi

# 3. 起 8-service compose
log_step "Bringing up docker compose stack under deploy/"
(cd "$DEPLOY_DIR" && docker compose up -d)
log_ok "Compose up; waiting for devops-toolkit to be ready"

# 4. 等 devops-toolkit ready(/api/v1/capabilities 不需 auth)
log_step "Waiting for http://localhost:3000/api/v1/capabilities to return 200"
for i in $(seq 1 60); do
	if curl -s -o /dev/null -w "%{http_code}" --max-time 2 \
		"$API_BASE/capabilities" 2>/dev/null | grep -q "^200$"; then
		log_ok "devops-toolkit ready after ${i}s"
		break
	fi
	if [ "$i" -eq 60 ]; then
		log_error "devops-toolkit did not become ready in 60s; check 'docker compose logs devops-toolkit'"
		exit 1
	fi
	sleep 1
done

# 5. 起 8 node containerlab
log_step "Deploying 8-node containerlab topology (first run pulls 2 images; ~30-60s)"
(cd "$DEPLOY_DIR" && containerlab deploy -t containerlab/topology.yml)

# 6. 读 topology-data.json,逐个 POST /devices + /physical-hosts
TOPO_JSON="$DEPLOY_DIR/containerlab/clab-devops-toolkit-clab/topology-data.json"
if [ ! -f "$TOPO_JSON" ]; then
	log_error "missing $TOPO_JSON after containerlab deploy"
	exit 1
fi

log_step "Reading IPs from topology-data.json and registering hosts"
# 物理 host 节点的 IP 在 .topology.nodes.<name>.interfaces[0].ipv4
# (mgmt 接口的 ipv4)。 network-multitool 节点 (core switch) 没有 sshd,
# 不 register 为 physical_host。
PHYSICAL_NODES=$(jq -r '
	.topology.nodes
	| to_entries[]
	| select(.value.labels.role == "physical_host")
	| .key
' "$TOPO_JSON")

for node in $PHYSICAL_NODES; do
	ip=$(jq -r --arg n "$node" '.topology.nodes[$n].interfaces[0].ipv4 // empty' "$TOPO_JSON")
	if [ -z "$ip" ]; then
		log_warn "no IP for $node; skipping"
		continue
	fi
	# 6a. POST /devices
	dev_code=$(curl -s -o /dev/null -w "%{http_code}" \
		-X POST -H "Content-Type: application/json" \
		-H "X-User: $API_USER" \
		-d "{\"id\":\"$node\",\"name\":\"$node\",\"type\":\"physical_host\",\"ip\":\"$ip\",\"dc\":\"$(echo $node | cut -d- -f1)\"}" \
		"$API_BASE/devices" || echo 000)
	if [ "$dev_code" -ge 200 ] && [ "$dev_code" -lt 300 ]; then
		log_ok "POST /devices $node -> $dev_code (ip=$ip)"
	else
		log_warn "POST /devices $node -> $dev_code"
	fi
	# 6b. POST /physical-hosts
	ph_code=$(curl -s -o /dev/null -w "%{http_code}" \
		-X POST -H "Content-Type: application/json" \
		-H "X-User: $API_USER" \
		-d "{\"device_id\":\"$node\",\"ip_address\":\"$ip\",\"ssh_user\":\"root\",\"ssh_port\":22}" \
		"$API_BASE/physical-hosts" || echo 000)
	if [ "$ph_code" -ge 200 ] && [ "$ph_code" -lt 300 ]; then
		log_ok "POST /physical-hosts $node -> $ph_code"
	else
		log_warn "POST /physical-hosts $node -> $ph_code"
	fi
done

# 7. 触发 1 次手动 probe 加速首波数据
log_step "Triggering 1 manual probe per host (so Grafana has data before the 1m tick)"
sleep 2
for node in $PHYSICAL_NODES; do
	ph_id=$(curl -s -H "X-User: $API_USER" "$API_BASE/physical-hosts" \
		| jq -r --arg n "$node" '.[] | select(.device_id==$n) | .id' | head -1)
	if [ -z "$ph_id" ]; then
		log_warn "no physical_host row for $node; skipping probe"
		continue
	fi
	pr_code=$(curl -s -o /dev/null -w "%{http_code}" \
		-X POST -H "X-User: $API_USER" \
		"$API_BASE/physical-hosts/$ph_id/probe" || echo 000)
	log_info "probe $node ($ph_id) -> $pr_code"
done

# 8. 报告
log_step "Dev env up. Access URLs:"
log_info "  Backend API:  $API_BASE (use header: X-User: $API_USER)"
log_info "  Grafana:      http://localhost:3001  (admin / admin)"
log_info "  Prometheus:   http://localhost:9090"
log_info "  Loki:         http://localhost:3100 (no UI; query via Grafana)"
log_info "  InfluxDB:     http://localhost:8086 (NOT in compose this session — skipped)"
log_info ""
log_info "SSH prober key (private, gitignored): $PROBER_KEY"
log_info "Prober key fingerprint:"
ssh-keygen -lf "$PROBER_KEY" || true
log_info ""
log_info "Wait 60s for the monitor loop to tick once, then:"
log_info "  curl -s -H 'X-User: $API_USER' $API_BASE/physical-hosts | jq '.[] | {device_id, state, consecutive_fails, last_check_at}'"
log_info "Open http://localhost:3001/d/devops-toolkit-ops-view in a browser."
log_ok "Done. dev-env-down.sh to teardown."
