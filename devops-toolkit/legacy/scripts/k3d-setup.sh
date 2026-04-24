#!/bin/bash
#
# K8s Multi-Cluster Setup Script using k3d
# Creates lightweight k3s clusters for testing
#
# k3d advantages over kind:
# - Faster cluster creation (seconds vs minutes)
# - Better memory efficiency
# - Easier multi-cluster management
# - Works well with Docker
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K3D_VERSION="${K3D_VERSION:-v5.7.4}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.28.0}"

# Cluster configurations: name:agents:apiPort
CLUSTERS=(
  "dev-cluster-1:3:31000"
  "dev-cluster-2:3:32000"
)

KUBECONFIG_DIR="${HOME}/.kube"

log() {
  echo "[$(date '+%H:%M:%S')] $*"
}

error() {
  echo "[$(date '+%H:%M:%S')] ERROR: $*" >&2
}

check_dependencies() {
  log "Checking dependencies..."

  # Check Docker
  if ! command -v docker &> /dev/null; then
    error "Docker is not installed"
    exit 1
  fi

  if ! docker info &> /dev/null; then
    error "Docker is not running"
    exit 1
  fi

  log "Docker is available: $(docker --version)"

  # Install k3d if not present
  if ! command -v k3d &> /dev/null; then
    log "Installing k3d ${K3D_VERSION}..."
    curl -sL "https://github.com/k3d-io/k3d/releases/download/${K3D_VERSION}/k3d-$(uname)-amd64" -o /tmp/k3d
    chmod +x /tmp/k3d
    sudo mv /tmp/k3d /usr/local/bin/k3d
    log "k3d installed"
  else
    log "k3d already installed: $(k3d version)"
  fi

  # Install kubectl if not present
  if ! command -v kubectl &> /dev/null; then
    log "Installing kubectl ${KUBECTL_VERSION}..."
    curl -sL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" -o /tmp/kubectl
    chmod +x /tmp/kubectl
    sudo mv /tmp/kubectl /usr/local/bin/kubectl
    log "kubectl installed"
  else
    log "kubectl already installed: $(kubectl version --client --short 2>/dev/null || kubectl version --client 2>/dev/null | head -1)"
  fi

  log "All dependencies available"
}

setup_kubeconfig_dir() {
  mkdir -p "$KUBECONFIG_DIR"
  chmod 700 "$KUBECONFIG_DIR"
  log "Kubeconfig directory: $KUBECONFIG_DIR"
}

# Create a k3d cluster with specified parameters
# Usage: create_cluster clusterName numAgents apiPort
create_cluster() {
  local cluster_name=$1
  local num_agents=${2:-3}
  local api_port=${3:-31000}

  log "Creating k3d cluster ${cluster_name} with ${num_agents} agents..."

  # Check if cluster already exists
  if k3d cluster list 2>/dev/null | grep -q "^${cluster_name} "; then
    log "Cluster ${cluster_name} already exists, skipping..."
    return 0
  fi

  # Create cluster with k3d
  # --agents: number of agent nodes
  # -p: port mappings (hostPort:containerPort@loadbalancer)
  # --k3s-arg: additional k3s arguments
  if k3d cluster create "${cluster_name}" \
    --agents "${num_agents}" \
    -p "${api_port}:6443@loadbalancer" \
    --k3s-arg "--disable=traefik@server:0" \
    --timeout 120s \
    --wait; then

    log "Cluster ${cluster_name} created successfully"

    # Export kubeconfig for this cluster
    local kubeconfig_path="${KUBECONFIG_DIR}/config-${cluster_name}"
    k3d kubeconfig get "${cluster_name}" > "${kubeconfig_path}"

    log "  Kubeconfig: ${kubeconfig_path}"
    log "  API Server: localhost:${api_port}"

    # Wait for cluster to be ready
    wait_for_cluster "${cluster_name}" "${kubeconfig_path}"

  else
    error "Failed to create cluster ${cluster_name}"
    return 1
  fi
}

# Wait for cluster to be ready
wait_for_cluster() {
  local cluster_name=$1
  local kubeconfig_path=$2

  log "Waiting for ${cluster_name} to be ready..."

  local max_attempts=30
  local attempt=0

  while [ $attempt -lt $max_attempts ]; do
    if kubectl --kubeconfig="${kubeconfig_path}" cluster-info &>/dev/null; then
      local nodes=$(kubectl --kubeconfig="${kubeconfig_path}" get nodes --no-headers 2>/dev/null | wc -l)
      log "  Cluster ready: ${nodes} nodes"
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 2
  done

  error "Cluster ${cluster_name} failed to become ready after ${max_attempts} attempts"
  return 1
}

# Delete a k3d cluster
delete_cluster() {
  local cluster_name=$1

  log "Deleting k3d cluster ${cluster_name}..."

  if k3d cluster delete "${cluster_name}"; then
    rm -f "${KUBECONFIG_DIR}/config-${cluster_name}"
    log "Cluster ${cluster_name} deleted"
  else
    error "Failed to delete cluster ${cluster_name}"
  fi
}

# List all k3d clusters
list_clusters() {
  log "Current k3d clusters:"
  echo ""

  k3d cluster list 2>/dev/null | while read line; do
    echo "  $line"
  done

  if ! k3d cluster list 2>/dev/null | grep -q .; then
    echo "  No clusters found"
  fi
}

# Check cluster health
health_check() {
  local cluster_name=$1
  local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

  if [ ! -f "$kubeconfig" ]; then
    error "kubeconfig not found for ${cluster_name}"
    return 1
  fi

  log "Health check for ${cluster_name}:"

  echo ""
  kubectl --kubeconfig="$kubeconfig" get nodes -o wide 2>/dev/null || error "Cannot get nodes"
  echo ""
  kubectl --kubeconfig="$kubeconfig" get pods -A 2>/dev/null | head -20 || error "Cannot get pods"
}

# Install metrics-server on a cluster
install_metrics_server() {
  local cluster_name=$1
  local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

  log "Installing metrics-server on ${cluster_name}..."

  kubectl --kubeconfig="$kubeconfig" apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml 2>/dev/null || true

  # Wait for metrics-server
  sleep 10

  if kubectl --kubeconfig="$kubeconfig" top nodes &>/dev/null; then
    log "Metrics available on ${cluster_name}"
    kubectl --kubeconfig="$kubeconfig" top nodes
  else
    log "Metrics not yet available (may take a few minutes)"
  fi
}

# Deploy sample nginx app
deploy_sample_app() {
  local cluster_name=$1
  local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

  log "Deploying sample nginx app on ${cluster_name}..."

  kubectl --kubeconfig="$kubeconfig" create deployment nginx-test --image=nginx:1.21 --replicas=2 --dry-run=client -o yaml | \
    kubectl --kubeconfig="$kubeconfig" apply -f -

  kubectl --kubeconfig="$kubeconfig" expose deployment nginx-test --port=80 --type=ClusterIP

  log "Sample app deployed on ${cluster_name}"

  kubectl --kubeconfig="$kubeconfig" get pods -l app=nginx 2>/dev/null || true
}

# Create merged kubeconfig for all clusters
setup_mixed_context() {
  log "Setting up mixed kubeconfig..."

  local mixed_config="${KUBECONFIG_DIR}/config-k3d-mixed"
  > "$mixed_config"

  for cluster_info in "${CLUSTERS[@]}"; do
    local cluster_name=$(echo "$cluster_info" | cut -d: -f1)
    local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

    if [ -f "$kubeconfig" ]; then
      KUBECONFIG="$kubeconfig:$mixed_config" kubectl config view --flatten > "${mixed_config}.tmp" 2>/dev/null || true
      [ -f "${mixed_config}.tmp" ] && mv "${mixed_config}.tmp" "$mixed_config"
    fi
  done

  if [ -f "$mixed_config" ]; then
    log "Mixed config created: ${mixed_config}"
  else
    error "Failed to create mixed config"
  fi
}

# Cleanup all clusters
cleanup_all() {
  log "Cleaning up all k3d clusters..."

  for cluster_info in "${CLUSTERS[@]}"; do
    local cluster_name=$(echo "$cluster_info" | cut -d: -f1)
    if k3d cluster list 2>/dev/null | grep -q "^${cluster_name} "; then
      delete_cluster "$cluster_name"
    fi
  done

  # Also cleanup any other k3d clusters that might have been created
  for cluster_name in $(k3d cluster list 2>/dev/null | grep -v NAME | awk '{print $1}'); do
    log "Deleting extra cluster: ${cluster_name}"
    k3d cluster delete "${cluster_name}" 2>/dev/null || true
    rm -f "${KUBECONFIG_DIR}/config-${cluster_name}"
  done

  rm -f "${KUBECONFIG_DIR}"/config-k3d-*

  log "Cleanup complete"
}

# Run integration tests against clusters
run_integration_tests() {
  log "Running integration tests against k3d clusters..."

  # Set kubeconfig to mixed context
  export KUBECONFIG="${KUBECONFIG_DIR}/config-k3d-mixed"

  # Run Jest with k3d tests
  cd /mnt/devops/devops-toolkit
  npx jest --config=test/jest.config.js tests/k8s_cluster_manager.test.js tests/k8s_multi_cluster.test.js --coverage=false --maxWorkers=1 2>&1 | tail -30

  log "Integration tests complete"
}

show_usage() {
  cat << EOF
Usage: $0 [command] [options]

Commands:
  setup         Create all dev clusters (default)
  create <name> Create a specific cluster
  delete <name> Delete a specific cluster
  list          List all clusters
  health <name> Check cluster health
  mixed         Create merged kubeconfig for all clusters
  metrics <name> Install metrics-server on cluster
  sample-app    Deploy sample nginx app to all clusters
  cleanup       Delete all clusters
  test          Run integration tests against clusters
  help          Show this help

Examples:
  $0 setup              # Create dev clusters
  $0 create dev-cluster-1   # Create single cluster
  $0 list               # List clusters
  $0 health dev-cluster-1   # Check cluster health
  $0 test               # Run integration tests
  $0 cleanup            # Delete all clusters

Environment Variables:
  K3D_VERSION       k3d version (default: v5.7.4)
  KUBECTL_VERSION   kubectl version (default: v1.28.0)

Prerequisites:
  - Docker must be running
  - k3d will be installed automatically if missing
  - kubectl will be installed automatically if missing

EOF
}

main() {
  case "${1:-setup}" in
    setup)
      check_dependencies
      setup_kubeconfig_dir

      for cluster_info in "${CLUSTERS[@]}"; do
        local cluster_name=$(echo "$cluster_info" | cut -d: -f1)
        local num_agents=$(echo "$cluster_info" | cut -d: -f2)
        local api_port=$(echo "$cluster_info" | cut -d: -f3)
        create_cluster "$cluster_name" "$num_agents" "$api_port"
      done

      setup_mixed_context

      log "Setup complete!"
      log ""
      log "To use a specific cluster:"
      log "  export KUBECONFIG=${KUBECONFIG_DIR}/config-dev-cluster-1"
      log ""
      log "To use all clusters (mixed context):"
      log "  export KUBECONFIG=${KUBECONFIG_DIR}/config-k3d-mixed"
      log ""
      log "To run tests:"
      log "  $0 test"
      ;;

    create)
      check_dependencies
      setup_kubeconfig_dir

      if [ -z "$2" ]; then
        error "Cluster name required"
        exit 1
      fi

      # Parse cluster config or use defaults
      local found=0
      for cluster_info in "${CLUSTERS[@]}"; do
        if [ "$(echo "$cluster_info" | cut -d: -f1)" = "$2" ]; then
          create_cluster "$2" "$(echo "$cluster_info" | cut -d: -f2)" "$(echo "$cluster_info" | cut -d: -f3)"
          found=1
          break
        fi
      done

      if [ "$found" = "0" ]; then
        # Create with default settings
        create_cluster "$2" 3 31000
      fi
      ;;

    delete)
      if [ -z "$2" ]; then
        error "Cluster name required"
        exit 1
      fi
      delete_cluster "$2"
      ;;

    list)
      list_clusters
      ;;

    health)
      if [ -z "$2" ]; then
        error "Cluster name required"
        exit 1
      fi
      health_check "$2"
      ;;

    mixed)
      setup_mixed_context
      ;;

    metrics)
      if [ -z "$2" ]; then
        error "Cluster name required"
        exit 1
      fi
      install_metrics_server "$2"
      ;;

    sample-app)
      for cluster_info in "${CLUSTERS[@]}"; do
        local cluster_name=$(echo "$cluster_info" | cut -d: -f1)
        deploy_sample_app "$cluster_name"
      done
      ;;

    cleanup)
      cleanup_all
      ;;

    test)
      run_integration_tests
      ;;

    help|--help|-h)
      show_usage
      ;;

    *)
      error "Unknown command: $1"
      show_usage
      exit 1
      ;;
  esac
}

main "$@"
