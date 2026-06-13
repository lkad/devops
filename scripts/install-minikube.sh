#!/usr/bin/env bash
#
# install-minikube.sh — C-子项目: minikube 上 install 验证 DevOps Toolkit chart
#
# 用法: ./scripts/install-minikube.sh [--reset]
#

set -euo pipefail

# Prerequisites check
which minikube >/dev/null 2>&1 || { echo "ERROR: minikube not installed (https://minikube.sigs.k8s.io/docs/start/)"; exit 1; }
which helm >/dev/null 2>&1 || { echo "ERROR: helm not installed (https://helm.sh/docs/intro/install/)"; exit 1; }
which kubectl >/dev/null 2>&1 || { echo "ERROR: kubectl not installed (https://kubernetes.io/docs/tasks/tools/)"; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="$SCRIPT_DIR/../deploy/helm"

if [[ "${1:-}" == "--reset" ]]; then
    echo "==> Resetting minikube"
    minikube delete || true
fi

echo "==> Starting minikube (4 CPU, 8GB RAM)"
minikube start --cpus=4 --memory=8g --driver=docker

echo "==> Building helm dependencies"
cd "$CHART_DIR"
helm dep update

echo "==> Installing devops-toolkit chart (umbrella, all sub-charts enabled)"
helm install devops . \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=devops.minikube.local \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  --set cert-manager-issuer.enabled=true \
  --set cert-manager-issuer.clusterIssuer.selfSigned.enabled=true \
  --set cert-manager-issuer.certificate.enabled=true \
  --set cert-manager-issuer.certificate.dnsName=devops.minikube.local \
  --set monitoring.enabled=true \
  --set monitoring.serviceMonitor.enabled=true \
  --set monitoring.podMonitor.enabled=true \
  --set network-policies.enabled=true

echo "==> Waiting for deployment to be ready (timeout 5min)"
kubectl wait --for=condition=available --timeout=300s \
  deployment/devops-toolkit -n default

echo "==> Setting up port-forward in background"
kubectl port-forward svc/devops-toolkit 8080:80 > /tmp/portforward.log 2>&1 &
PORT_FORWARD_PID=$!
sleep 3

echo "==> Testing endpoints"
echo "  /live:"
curl -s -o /dev/null -w "    HTTP %{http_code}\n" http://localhost:8080/live
echo "  /ready:"
curl -s -o /dev/null -w "    HTTP %{http_code}\n" http://localhost:8080/ready
echo "  /api/v1/capabilities:"
curl -s -o /dev/null -w "    HTTP %{http_code}\n" http://localhost:8080/api/v1/capabilities

echo "==> Verifying sub-chart resources"
echo "  ClusterIssuer:"
kubectl get clusterissuer -o name 2>/dev/null || echo "    (none — external operator not installed)"
echo "  ServiceMonitor:"
kubectl get servicemonitor -o name 2>/dev/null || echo "    (none — prometheus-operator not installed)"
echo "  NetworkPolicy:"
kubectl get networkpolicy -l app.kubernetes.io/name=devops-toolkit -o name 2>/dev/null || echo "    (none)"

echo "==> Done! Access at http://localhost:8080"
echo "==> Stop port-forward: kill $PORT_FORWARD_PID"
echo "==> minikube dashboard: minikube dashboard"
