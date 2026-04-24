#!/bin/bash
#
# K8s Multi-Cluster Setup Script using kind
# Creates multiple kind clusters with multiple nodes for testing
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIND_VERSION="${KIND_VERSION:-v0.20.0}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.28.0}"

# Cluster configurations
CLUSTERS=(
  "cluster-1:3:31000"
  "cluster-2:3:32000"
  "cluster-3:2:33000"
)
# Format: name:nodes:apiServerPort

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

  log "Docker is available"

  # Install kind if not present
  if ! command -v kind &> /dev/null; then
    log "Installing kind ${KIND_VERSION}..."
    curl -Lo /tmp/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-$(uname)-amd64"
    chmod +x /tmp/kind
    sudo mv /tmp/kind /usr/local/bin/kind
  fi

  # Install kubectl if not present
  if ! command -v kubectl &> /dev/null; then
    log "Installing kubectl ${KUBECTL_VERSION}..."
    curl -Lo /tmp/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
    chmod +x /tmp/kubectl
    sudo mv /tmp/kubectl /usr/local/bin/kubectl
  fi

  log "All dependencies are available"
}

install_kind() {
  log "Installing kind..."
  curl -Lo /tmp/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-$(uname)-amd64"
  chmod +x /tmp/kind
  sudo mv /tmp/kind /usr/local/bin/kind
  log "kind installed"
}

install_kubectl() {
  log "Installing kubectl..."
  curl -Lo /tmp/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
  chmod +x /tmp/kubectl
  sudo mv /tmp/kubectl /usr/local/bin/kubectl
  log "kubectl installed"
}

create_kind_config() {
  local cluster_name=$1
  local num_nodes=$2
  local api_port=$3
  local config_file="/tmp/kind-config-${cluster_name}.yaml"

  cat > "$config_file" << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${cluster_name}
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: ${api_port}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    metadata:
      name: config
    networking:
      dnsDomain: cluster.local
      podSubnet: 10.24${api_port:2:1}.0.0/16
      serviceSubnet: 10.96.0.0/12
- role: worker
- role: worker
- role: worker
EOF

  # Add extra workers if needed
  if [ "$num_nodes" -gt 3 ]; then
    for ((i=4; i<=num_nodes; i++)); do
      echo "  - role: worker" >> "$config_file"
    done
  fi

  echo "$config_file"
}

setup_kubeconfig_dir() {
  mkdir -p "$KUBECONFIG_DIR"
  chmod 700 "$KUBECONFIG_DIR"
  log "Kubeconfig directory: $KUBECONFIG_DIR"
}

create_cluster() {
  local cluster_info=$1
  local cluster_name=$(echo "$cluster_info" | cut -d: -f1)
  local num_nodes=$(echo "$cluster_info" | cut -d: -f2)
  local api_port=$(echo "$cluster_info" | cut -d: -f3)

  log "Creating cluster ${cluster_name} with ${num_nodes} nodes..."

  # Check if cluster already exists
  if kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"; then
    log "Cluster ${cluster_name} already exists, skipping..."
    return 0
  fi

  # Create config file
  local config_file=$(create_kind_config "$cluster_name" "$num_nodes" "$api_port")

  # Create cluster
  if kind create cluster --name "$cluster_name" --config "$config_file" --wait 120s; then
    # Export kubeconfig
    local kubeconfig_path="${KUBECONFIG_DIR}/config-${cluster_name}"
    kind get kubeconfig --name "$cluster_name" > "$kubeconfig_path"

    log "Cluster ${cluster_name} created successfully"
    log "  Kubeconfig: ${kubeconfig_path}"

    # Verify nodes
    local nodes=$(kubectl --kubeconfig="$kubeconfig_path" get nodes --no-headers 2>/dev/null | wc -l)
    log "  Nodes: ${nodes}"

    # Cleanup config
    rm -f "$config_file"
  else
    error "Failed to create cluster ${cluster_name}"
    rm -f "$config_file"
    return 1
  fi
}

delete_cluster() {
  local cluster_name=$1

  log "Deleting cluster ${cluster_name}..."

  if kind delete cluster --name "$cluster_name"; then
    rm -f "${KUBECONFIG_DIR}/config-${cluster_name}"
    log "Cluster ${cluster_name} deleted"
  else
    error "Failed to delete cluster ${cluster_name}"
  fi
}

list_clusters() {
  log "Current kind clusters:"
  echo ""
  kind get clusters 2>/dev/null | while read cluster; do
    local kubeconfig="${KUBECONFIG_DIR}/config-${cluster}"
    local status="unknown"

    if [ -f "$kubeconfig" ]; then
      local nodes=$(kubectl --kubeconfig="$kubeconfig" get nodes --no-headers 2>/dev/null | wc -l)
      status="running (${nodes} nodes)"
    fi

    echo "  ${cluster}: ${status}"
  done

  if ! kind get clusters 2>/dev/null | grep -q .; then
    echo "  No clusters found"
  fi
}

health_check() {
  local cluster_name=$1
  local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

  if [ ! -f "$kubeconfig" ]; then
    echo "  kubeconfig not found"
    return 1
  fi

  kubectl --kubeconfig="$kubeconfig" get nodes 2>/dev/null | while read line; do
    echo "  $line"
  done
}

setup_mixed_context() {
  log "Setting up mixed kubeconfig..."

  local mixed_config="${KUBECONFIG_DIR}/config-kind-mixed"
  > "$mixed_config"

  for cluster_info in "${CLUSTERS[@]}"; do
    local cluster_name=$(echo "$cluster_info" | cut -d: -f1)
    local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

    if [ -f "$kubeconfig" ]; then
      # Use kubectl to merge configs
      KUBECONFIG="$kubeconfig:$mixed_config" kubectl config view --flatten > "${mixed_config}.tmp" 2>/dev/null
      mv "${mixed_config}.tmp" "$mixed_config"
    fi
  done

  log "Mixed config created: ${mixed_config}"
}

install_metrics_server() {
  local cluster_name=$1
  local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

  log "Installing metrics-server on ${cluster_name}..."

  kubectl --kubeconfig="$kubeconfig" apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml 2>/dev/null || true

  # Wait for metrics-server to be ready
  sleep 5

  kubectl --kubeconfig="$kubeconfig" top nodes 2>/dev/null && log "Metrics available" || log "Metrics not yet available"
}

deploy_sample_app() {
  local cluster_name=$1
  local kubeconfig="${KUBECONFIG_DIR}/config-${cluster_name}"

  log "Deploying sample nginx app on ${cluster_name}..."

  cat << EOF | kubectl --kubeconfig="$kubeconfig" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
  labels:
    app: nginx
    cluster: ${cluster_name}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.21
        ports:
        - containerPort: 80
EOF

  kubectl --kubeconfig="$kubeconfig" expose deployment nginx-test --port=80 --type=ClusterIP 2>/dev/null

  log "Sample app deployed on ${cluster_name}"
}

cleanup_all() {
  log "Cleaning up all clusters..."

  for cluster_info in "${CLUSTERS[@]}"; do
    local cluster_name=$(echo "$cluster_info" | cut -d: -f1)
    if kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"; then
      delete_cluster "$cluster_name"
    fi
  done

  rm -f "${KUBECONFIG_DIR}"/config-cluster-*
  log "Cleanup complete"
}

show_usage() {
  cat << EOF
Usage: $0 [command] [options]

Commands:
  setup         Create all clusters (default)
  create <name> Create a specific cluster
  delete <name> Delete a specific cluster
  list          List all clusters
  health <name> Check cluster health
  mixed         Create merged kubeconfig for all clusters
  metrics       Install metrics-server on all clusters
  sample-app    Deploy sample app to all clusters
  cleanup       Delete all clusters
  help          Show this help

Examples:
  $0 setup              # Create all clusters
  $0 create cluster-1   # Create cluster-1 only
  $0 list               # List all clusters
  $0 health cluster-1   # Check cluster-1 health

Environment Variables:
  KIND_VERSION    kind version (default: v0.20.0)
  KUBECTL_VERSION kubectl version (default: v1.28.0)

EOF
}

main() {
  case "${1:-setup}" in
    setup)
      check_dependencies
      setup_kubeconfig_dir

      for cluster_info in "${CLUSTERS[@]}"; do
        create_cluster "$cluster_info"
      done

      setup_mixed_context

      log "Setup complete!"
      log ""
      log "To use a specific cluster:"
      log "  export KUBECONFIG=${KUBECONFIG_DIR}/config-cluster-1"
      log ""
      log "To use all clusters:"
      log "  export KUBECONFIG=${KUBECONFIG_DIR}/config-kind-mixed"
      ;;
    create)
      check_dependencies
      setup_kubeconfig_dir

      if [ -z "$2" ]; then
        error "Cluster name required"
        exit 1
      fi

      # Find cluster config or use defaults
      local found=0
      for cluster_info in "${CLUSTERS[@]}"; do
        if [ "$(echo "$cluster_info" | cut -d: -f1)" = "$2" ]; then
          create_cluster "$cluster_info"
          found=1
          break
        fi
      done

      if [ "$found" = "0" ]; then
        # Create with default settings
        create_cluster "${2}:3:3${RANDOM:0:3}"
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
      for cluster_info in "${CLUSTERS[@]}"; do
        install_metrics_server "$(echo "$cluster_info" | cut -d: -f1)"
      done
      ;;
    sample-app)
      for cluster_info in "${CLUSTERS[@]}"; do
        deploy_sample_app "$(echo "$cluster_info" | cut -d: -f1)"
      done
      ;;
    cleanup)
      cleanup_all
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