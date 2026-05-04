#!/bin/bash
# Deploy Promtail to k3d clusters to collect K8s pod logs to Loki

set -e

PROMTAIL_CONFIG="/mnt/devops/configs/promtail/promtail-k8s-config.yaml"
KUBECONFIG_CLUSTER1="$HOME/.kube/config-dev-cluster-1"
KUBECONFIG_CLUSTER2="$HOME/.kube/config-dev-cluster-2"

# Create Promtail namespace
create_promtail_ns() {
    local kubeconfig=$1
    local cluster=$2

    echo "Creating promtail namespace on $cluster..."
    kubectl --kubeconfig="$kubeconfig" create namespace promtail --dry-run=client -o yaml | kubectl --kubeconfig="$kubeconfig" apply -f -
}

# Create configmap for Promtail
create_promtail_configmap() {
    local kubeconfig=$1
    local cluster=$2

    echo "Creating Promtail configmap on $cluster..."
    kubectl --kubeconfig="$kubeconfig" -n promtail delete configmap promtail-config --ignore-not-found=true
    kubectl --kubeconfig="$kubeconfig" -n promtail create configmap promtail-config \
        --from-file=promtail.yaml="$PROMTAIL_CONFIG"
}

# Create Promtail DaemonSet
create_promtail_daemonset() {
    local kubeconfig=$1
    local cluster=$2
    local loki_url=${3:-http://192.168.121.111:3100/loki/api/v1/push}

    echo "Deploying Promtail DaemonSet on $cluster..."
    echo "Loki URL: $loki_url"
    kubectl --kubeconfig="$kubeconfig" -n promtail apply -f - << EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: promtail
  labels:
    app: promtail
spec:
  selector:
    matchLabels:
      app: promtail
  template:
    metadata:
      labels:
        app: promtail
    spec:
      serviceAccountName: promtail
      containers:
        - name: promtail
          image: grafana/promtail:2.9.4
          args:
            - -config.file=/etc/promtail/promtail.yaml
            - -client.url=${loki_url}
          securityContext:
            privileged: true
            runAsUser: 0
          volumeMounts:
            - name: config
              mountPath: /etc/promtail
            - name: varlog
              mountPath: /var/log
            - name: varlibdockercontainers
              mountPath: /var/lib/docker/containers
              readOnly: true
            - name: kubelet
              mountPath: /var/lib/kubelet
              readOnly: true
          resources:
            limits:
              cpu: 200m
              memory: 256Mi
            requests:
              cpu: 100m
              memory: 128Mi
      volumes:
        - name: config
          configMap:
            name: promtail-config
        - name: varlog
          hostPath:
            path: /var/log
        - name: varlibdockercontainers
          hostPath:
            path: /var/lib/docker/containers
        - name: kubelet
          hostPath:
            path: /var/lib/kubelet
EOF
}

# Create service account for Promtail
create_service_account() {
    local kubeconfig=$1
    local cluster=$2

    echo "Creating Promtail service account on $cluster..."
    kubectl --kubeconfig="$kubeconfig" -n promtail delete serviceaccount promtail --ignore-not-found=true
    kubectl --kubeconfig="$kubeconfig" -n promtail create serviceaccount promtail

    # Create cluster role binding
    kubectl --kubeconfig="$kubeconfig" delete clusterrolebinding promtail 2>/dev/null || true
    kubectl --kubeconfig="$kubeconfig" create clusterrolebinding promtail \
        --clusterrole=cluster-admin \
        --serviceaccount=promtail:promtail
}

# Wait for Promtail to be ready
wait_for_promtail() {
    local kubeconfig=$1
    local cluster=$2

    echo "Waiting for Promtail DaemonSet on $cluster..."
    kubectl --kubeconfig="$kubeconfig" -n promtail rollout status daemonset/promtail --timeout=60s || true
    kubectl --kubeconfig="$kubeconfig" -n promtail get pods
}

# Main deployment
deploy_to_cluster() {
    local kubeconfig=$1
    local cluster=$2
    local loki_url=${3:-http://192.168.121.111:3100/loki/api/v1/push}

    echo "=========================================="
    echo "Deploying Promtail to $cluster"
    echo "=========================================="

    create_promtail_ns "$kubeconfig" "$cluster"
    create_service_account "$kubeconfig" "$cluster"
    create_promtail_configmap "$kubeconfig" "$cluster"
    create_promtail_daemonset "$kubeconfig" "$cluster" "$loki_url"
    wait_for_promtail "$kubeconfig" "$cluster"

    echo "Promtail deployed to $cluster successfully!"
    echo ""
}

# Deploy to both clusters with Loki at host IP
deploy_to_cluster "$KUBECONFIG_CLUSTER1" "dev-cluster-1" "http://192.168.121.111:3100/loki/api/v1/push"
deploy_to_cluster "$KUBECONFIG_CLUSTER2" "dev-cluster-2" "http://192.168.121.111:3100/loki/api/v1/push"

echo "=========================================="
echo "Promtail deployment complete!"
echo "=========================================="
echo ""
echo "Checking Loki connectivity from pods..."
kubectl --kubeconfig="$KUBECONFIG_CLUSTER1" -n promtail run curl-pod --image=curlimages/curl --rm -i --restart=Never -- curl -s http://192.168.121.111:3100/ready
