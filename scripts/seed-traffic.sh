#!/usr/bin/env bash
# scripts/seed-traffic.sh — populate the dev environment with
# realistic data so the Grafana dashboards show non-empty panels.
#
# Generates three kinds of data:
#   1. HTTP request metrics  — curl a representative set of
#      /api/v1/* endpoints 200+ times, so http_requests_total
#      and the latency histograms fill in
#   2. Pipeline runs         — POST /pipelines then trigger
#      /pipelines/:id/trigger; runs fail (no real CI) but the
#      pipeline_runs table + Grafana panel show real failure
#      patterns
#   3. Service health        — POST /services then poll
#      /services/:id/health to populate the
#      service_health_status gauge with mixed healthy/degraded
#
# Idempotent-ish: re-running will duplicate some rows (the POST
# /devices and POST /services are not dedup-keyed here) but
# won't break the dashboards. The /capabilities and /physical-hosts
# GETs are idempotent.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/log.sh
source "$SCRIPT_DIR/lib/log.sh"
log_init "$(basename "${BASH_SOURCE[0]}")"

API_BASE="${API_BASE:-http://localhost:3000/api/v1}"
API_USER="${API_USER:-alice}"

log_step "Seeding traffic against $API_BASE (dev_bypass X-User: $API_USER)"

# -----------------------------------------------------------------------------
# 1. HTTP request metrics
#
# We hit a representative set of /api/v1/* endpoints repeatedly. The mix
# matters: a single endpoint hit 1000x would give one route a huge
# count and the top-10 panel would only show that one route. We
# rotate through ~20 endpoints so the top-10 panel fills with
# meaningful variety.
# -----------------------------------------------------------------------------
log_step "Generating HTTP traffic (200 requests across ~20 endpoints)"

ENDPOINTS=(
  "GET /capabilities"
  "GET /physical-hosts"
  "GET /physical-hosts?limit=10"
  "GET /physical-hosts?limit=50"
  "GET /devices"
  "GET /devices?limit=10"
  "GET /devices?limit=50"
  "GET /services"
  "GET /services?limit=10"
  "GET /pipelines"
  "GET /pipelines?limit=10"
  "GET /projects"
  "GET /projects?limit=10"
  "GET /alerts"
  "GET /audit"
  "GET /audit?limit=20"
  "GET /logs/capabilities"
  "GET /logs/saved-filters"
  "GET /logs/retention"
  "GET /metrics"
  "GET /alerts/channels"
  "GET /alerts/history"
  "GET /discovery/runs"
)

# Some endpoints return 401 because the dev_bypass header is
# checked at the auth middleware but specific routes (like
# /api/v1/services/:id/health) require write perms in some
# flows. The 401s and 404s are exactly what we want — they
# populate the 4xx bucket and exercise error paths.
for i in $(seq 1 8); do
  for ep in "${ENDPOINTS[@]}"; do
    method=$(echo "$ep" | cut -d' ' -f1)
    path=$(echo "$ep" | cut -d' ' -f2)
    curl -s -o /dev/null \
      -X "$method" \
      -H "X-User: $API_USER" \
      "${API_BASE}${path}" >/dev/null || true
  done
done
log_ok "  8 rounds × ${#ENDPOINTS[@]} endpoints = ~$((8 * ${#ENDPOINTS[@]})) requests"

# -----------------------------------------------------------------------------
# 2. Pipeline runs
#
# Create 4 pipelines with different strategies, then trigger each
# one 3-5 times. Runs will fail (no real CI executor in dev) but
# the pipeline_runs table + dashboard will show realistic
# success/failure distribution.
# -----------------------------------------------------------------------------
log_step "Seeding pipelines and triggering runs"

create_pipeline() {
  local name="$1"
  local strategy="$2"
  # target_type must be one of project / device / cluster / webhook
  # (see internal/pipeline/models.go TargetType.Valid). The Steps
  # field is required; the Go struct uses json:"步骤" but the
  # binding only works with the English key "steps" in practice —
  # see the comments at pipelineRequest (handler.go:55).
  curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Content-Type: application/json" -H "X-User: $API_USER" \
    -d "{\"name\":\"$name\",\"project_id\":\"proj-order-backend\",\"target_type\":\"device\",\"target_id\":\"dev-dc1-web-21\",\"trigger\":\"manual\",\"strategy\":\"$strategy\",\"steps\":[{\"name\":\"checkout\",\"type\":\"shell\",\"config\":{\"command\":\"git clone\"}}]}" \
    "${API_BASE}/pipelines" || echo "000"
}

trigger_pipeline() {
  local name="$1"
  local id
  id=$(curl -s -H "X-User: $API_USER" "${API_BASE}/pipelines" \
    | python3 -c "import json, sys; d=json.load(sys.stdin); rows=[p for p in d.get('data',[]) if p.get('id')=='$name']; print(rows[0]['id'] if rows else '')" 2>/dev/null || true)
  if [ -z "$id" ]; then
    log_warn "  $name: not found, skipping trigger"
    return
  fi
  curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "X-User: $API_USER" \
    "${API_BASE}/pipelines/$id/trigger" || echo "000"
}

create_pipeline "build-and-deploy" "blue-green"
create_pipeline "rollout-canary" "canary"
create_pipeline "hotfix-rolling" "rolling"
create_pipeline "nightly-e2e" "blue-green"

for pipe in build-and-deploy rollout-canary hotfix-rolling nightly-e2e; do
  for i in 1 2 3 4 5; do
    trigger_pipeline "$pipe" >/dev/null
  done
done
log_ok "  4 pipelines created, 5 triggers each = 20 runs"

# -----------------------------------------------------------------------------
# 3. Service health
#
# Create 4 services with mixed tiers. The /services/:id/health
# endpoint updates the service_health_status gauge. We poll each
# service once to populate the gauge (defaults to 1=healthy on
# the first hit since there's no real health source wired up).
# -----------------------------------------------------------------------------
log_step "Seeding services for service_health_status metric"

create_service() {
  local id="$1" tier="$2" derived="$3"
  curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Content-Type: application/json" -H "X-User: $API_USER" \
    -d "{\"id\":\"$id\",\"name\":\"$id\",\"tier\":\"$tier\",\"owner\":\"sre@example.com\",\"repository_url\":\"https://github.com/your-org/$id\"}" \
    "${API_BASE}/services" || echo "000"
}

poll_health() {
  local id="$1"
  curl -s -o /dev/null -H "X-User: $API_USER" \
    "${API_BASE}/services/$id/health" || true
}

create_service "order-backend"  "critical" "k8s_pod_health"
create_service "payment-api"    "critical" "k8s_pod_health"
create_service "user-portal"    "standard" "last_pipeline_run"
create_service "analytics-svc"  "standard" "last_pipeline_run"
create_service "internal-tool"  "internal" "manual"

# 3 polls each, ~1s apart, to populate the gauge and the
# service_health_rollup_total counter
for svc in order-backend payment-api user-portal analytics-svc internal-tool; do
  for i in 1 2 3; do
    poll_health "$svc"
    sleep 0.3
  done
done
log_ok "  5 services created + health polled"

# -----------------------------------------------------------------------------
# 4. Register a real k8s cluster with encrypted kubeconfig.
#
# Pulls the active k3d context from `kubectl config view --raw`
# and POSTs it to /api/v1/k8s/clusters with all the fields.
# The handler encrypts the kubeconfig (AES-GCM via K8S_CRYPTO_KEY)
# before persisting to k8s_clusters.kubeconfig_encrypted.
#
# Idempotency: the cluster name is unique-indexed, so re-running
# seed-traffic.sh returns 409 Conflict. We detect that and skip.
# -----------------------------------------------------------------------------
log_step "Registering active k3d cluster (k8s_clusters row + encrypted kubeconfig)"
if ! command -v kubectl >/dev/null 2>&1; then
  log_warn "  kubectl not in PATH; skipping k8s cluster registration"
else
  KUBECONFIG_RAW=$(kubectl config view --raw 2>/dev/null || echo "")
  K8S_NAME=$(kubectl config current-context 2>/dev/null || echo "")
  K8S_API=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || echo "")
  if [ -z "$KUBECONFIG_RAW" ] || [ -z "$K8S_API" ]; then
    log_warn "  no active kubectl context; skipping"
  else
    # Build JSON payload. Use python for proper escaping of the
    # multi-line kubeconfig YAML.
    PAYLOAD=$(KUBECONFIG="$KUBECONFIG_RAW" K8S_NAME="$K8S_NAME" K8S_API="$K8S_API" \
      python3 -c '
import json, os
print(json.dumps({
  "name": os.environ["K8S_NAME"],
  "type": "k3d",
  "api_server": os.environ["K8S_API"],
  "kubeconfig": os.environ["KUBECONFIG"],
  "in_cluster": False,
}))')
    CODE=$(curl -s -o /tmp/k8s-resp.json -w "%{http_code}" \
      -X POST -H "Content-Type: application/json" -H "X-User: $API_USER" \
      -d "$PAYLOAD" "${API_BASE}/k8s/clusters" || echo "000")
    if [ "$CODE" = "201" ]; then
      log_ok "  registered $K8S_NAME → $K8S_API"
    elif [ "$CODE" = "409" ]; then
      log_info "  $K8S_NAME already registered (409 Conflict, expected on re-run)"
    else
      log_warn "  POST /k8s/clusters -> $CODE  (response below)"
      cat /tmp/k8s-resp.json | head -1
    fi
  fi
fi

# -----------------------------------------------------------------------------
# 5. Final traffic burst — one more round to capture the
# post-seed state. This is the data the dashboard will show.
# -----------------------------------------------------------------------------
log_step "Final traffic burst to capture post-seed state"
for ep in "${ENDPOINTS[@]}"; do
  method=$(echo "$ep" | cut -d' ' -f1)
  path=$(echo "$ep" | cut -d' ' -f2)
  for i in 1 2 3; do
    curl -s -o /dev/null \
      -X "$method" -H "X-User: $API_USER" \
      "${API_BASE}${path}" >/dev/null || true
  done
done
log_ok "  3 rounds × ${#ENDPOINTS[@]} endpoints = $((3 * ${#ENDPOINTS[@]})) requests"

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
log_ok "Seed complete. Inspect:"
log_info "  http_requests_total:     curl -s http://localhost:3000/metrics | grep http_requests_total | head -10"
log_info "  pipeline_runs:           psql -U devops -d devops -c 'select status, count(*) from pipeline_runs group by status;'"
log_info "  service_health_status:  curl -s http://localhost:9090/api/v1/query?query=devops_toolkit_service_health_status | jq ."
log_info "  audit_logs:              psql -U devops -d devops -c 'select count(*), action from audit_logs group by action;'"
log_info ""
log_info "Wait 60-90s for the Prometheus scrape interval (15s × 4) + monitor loop 1m tick, then refresh Grafana."
