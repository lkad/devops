# DevOps Toolkit Helm Chart — Testing Guide

## Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/) >= 1.30
- [helm](https://helm.sh/docs/intro/install/) >= 3.12
- [kubectl](https://kubernetes.io/docs/tasks/tools/) >= 1.27
- 4 CPU, 8GB RAM available
- Docker (minikube driver)

## Quick Start (minikube)

```bash
./scripts/install-minikube.sh
```

This script:
1. Starts minikube (4 CPU, 8GB RAM)
2. Builds helm dependencies (`helm dep update`)
3. Installs devops-toolkit with all 5 sub-charts enabled
4. Waits for deployment to be ready (5min timeout)
5. Sets up port-forward in background
6. Tests `/live`, `/ready`, `/api/v1/capabilities`
7. Verifies sub-chart resources

## Manual Test Steps

1. `minikube start --cpus=4 --memory=8g`
2. `cd deploy/helm && helm dep update`
3. `helm install devops .` (with --set flags for enabled sub-charts)
4. `kubectl wait --for=condition=available --timeout=300s deployment/devops-toolkit`
5. `kubectl port-forward svc/devops-toolkit 8080:80`
6. Open http://localhost:8080 in browser

## Verify Each Sub-Chart

```bash
# cert-manager-issuer (requires cert-manager operator)
kubectl get clusterissuer
kubectl get certificate

# external-secrets (requires external-secrets operator)
kubectl get secretstore
kubectl get externalsecret

# sealed-secrets (requires sealed-secrets controller)
kubectl get sealedsecret

# monitoring (requires prometheus-operator)
kubectl get servicemonitor
kubectl get podmonitor

# network-policies
kubectl get networkpolicy -l app.kubernetes.io/name=devops-toolkit
```

## Endpoints

- `GET /live` — liveness (always 200 if process up)
- `GET /ready` — readiness (200 if all dependencies up; 503 if degraded)
- `GET /api/v1/capabilities` — public route, returns the auth + RBAC matrix
- `GET /api/v1/projects` — requires auth (returns 401 if no JWT)

## Cleanup

```bash
helm uninstall devops
minikube delete
```

## CI

- `helm template` + `helm lint` run on every PR (via `.github/workflows/ci.yml`).
- minikube install is **NOT** in CI (resource constraints). Run locally for full E2E.
