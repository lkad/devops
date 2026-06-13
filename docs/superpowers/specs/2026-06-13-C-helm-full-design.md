# C 子项目 — Helm chart 完整化 设计

> **范围**:umbrella chart 完整化 — cert-manager ClusterIssuer + ExternalSecret + SealedSecret + ServiceMonitor/PodMonitor + NetworkPolicy + minikube install 验证脚本。
>
> **日期**: 2026-06-13
> **作者**: Claude
> **状态**: Design (待用户 review)
> **对应子项目**: C(5 子项目分解的第三个,接 D 之后)
> **总估时**: 1 周
> **前序**: A 子项目(v0.2.1.0) 已交付 Helm chart 骨架(13 模板);D 子项目(v0.4.0.0) 已落地 UI 补全 + CORS

---

## 1. 背景与动机

### 1.1 现状(v0.4.0.0)

| 现状 | 详情 |
|---|---|
| **Helm chart 骨架** | `deploy/helm/` 13 模板(Chart.yaml + values.yaml + 9 templates + _helpers.tpl + README + symlink) |
| **cert-manager partial** | ingress annotation `cert-manager.io/cluster-issuer: letsencrypt-prod` 已设,但 ClusterIssuer CRD 不存在 |
| **ExternalSecret 占位** | `templates/secret.yaml` 注释提到 ExternalSecret,但 CRD 不在 chart |
| **SealedSecret** | 不在 chart |
| **ServiceMonitor / PodMonitor** | 不在 chart,Prometheus 需自己 scrape /metrics |
| **NetworkPolicy** | 不在 chart,默认全开放 |
| **minikube 验证** | 脚本不存在,文档不完整 |

### 1.2 已知缺口

| 项 | 估时 |
|---|---|
| cert-manager ClusterIssuer + Certificate CRD | 0.5 day |
| ExternalSecret + SecretStore (AWS 示例) | 0.5 day |
| SealedSecret CRD (静态加密,云无关) | 0.5 day |
| ServiceMonitor + PodMonitor (Prometheus 抓 /metrics) | 0.5 day |
| NetworkPolicy (默认 deny + explicit allow) | 0.5 day |
| minikube install 脚本 + TESTING.md | 1 day |
| **主 Chart.yaml dependencies 接线** | 0.5 day |
| **README 更新 + 文档收尾** | 0.5 day |

总估时 ~4-5 天,1 周内可完成。

### 1.3 目标

5 个子 chart(cert-manager-issuer + external-secrets + sealed-secrets + monitoring + network-policies) + 主 chart dependencies 接线 + minikube install 脚本,1 周内完成。

---

## 2. 架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│        C 子项目 — Helm 完整化 (umbrella chart)                  │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Helm chart 树:                                                  │
│  deploy/helm/                                                    │
│   ├─ Chart.yaml                  (umbrella, dependencies list)  │
│   ├─ values.yaml                 (主 values)                    │
│   ├─ templates/                  (13 templates, A 子项目保留) │
│   └─ charts/                                                    │
│       ├─ cert-manager-issuer/                                  │
│       │   ├─ Chart.yaml                                         │
│       │   └─ templates/cluster-issuer.yaml                      │
│       │   └─ templates/certificate.yaml                        │
│       ├─ external-secrets/                                     │
│       │   ├─ Chart.yaml                                         │
│       │   └─ templates/secret-store.yaml                       │
│       │   └─ templates/external-secret.yaml                    │
│       ├─ sealed-secrets/                                      │
│       │   ├─ Chart.yaml                                         │
│       │   └─ templates/sealed-secret.yaml                      │
│       ├─ monitoring/                                           │
│       │   ├─ Chart.yaml                                         │
│       │   └─ templates/service-monitor.yaml                    │
│       │   └─ templates/pod-monitor.yaml                       │
│       └─ network-policies/                                    │
│           ├─ Chart.yaml                                         │
│           └─ templates/network-policy.yaml                      │
│                                                                  │
│  minikube 验证 (scripts/install-minikube.sh):                 │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  minikube start --cpus=4 --memory=8g                     │    │
│  │  helm dep update deploy/helm/                            │    │
│  │  helm install devops deploy/helm/                         │    │
│  │  kubectl wait --for=condition=available                   │    │
│  │  kubectl port-forward svc/devops-toolkit 8080:80         │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  文档:                                                           │
│  ├─ deploy/helm/TESTING.md          (minikube 验证步骤)         │
│  ├─ deploy/helm/README.md           (更新,含 5 个子 chart 说明) │
│  └─ deploy/helm/charts/<sub>/README.md (子 chart 文档)       │
└──────────────────────────────────────────────────────────────────┘
```

### 关键设计原则

1. **umbrella chart 风格** — 业界标准,helm dep update 一键拉所有子 chart
2. **每个子 chart 独立可启** — `helm install devops-externalsecrets deploy/helm/charts/external-secrets/` 单独启用
3. **主 chart values 引用子 chart values** — 单一来源
4. **cert-manager partial 集成** — 已有 ingress annotation,新增 ClusterIssuer CRD
5. **minikube 文档化,CI 不跑**(资源限制)
6. **YAGNI** — Velero / ArgoCD / Vault 留后续

---

## 3. 关键 Component 详解

### 3.1 cert-manager-issuer 子 chart (Agent 1)

**Files:**
- `deploy/helm/charts/cert-manager-issuer/Chart.yaml`
- `deploy/helm/charts/cert-manager-issuer/values.yaml`
- `deploy/helm/charts/cert-manager-issuer/templates/cluster-issuer.yaml`
- `deploy/helm/charts/cert-manager-issuer/templates/certificate.yaml`
- `deploy/helm/charts/cert-manager-issuer/templates/_helpers.tpl`
- `deploy/helm/charts/cert-manager-issuer/README.md`

**主 Chart.yaml** (deploy/helm/Chart.yaml) 加 dependency:
```yaml
dependencies:
  - name: cert-manager-issuer
    version: "0.1.0"
    repository: "file://charts/cert-manager-issuer"
    condition: cert-manager-issuer.enabled
  - name: external-secrets
    version: "0.1.0"
    repository: "file://charts/external-secrets"
    condition: external-secrets.enabled
  - name: sealed-secrets
    version: "0.1.0"
    repository: "file://charts/sealed-secrets"
    condition: sealed-secrets.enabled
  - name: monitoring
    version: "0.1.0"
    repository: "file://charts/monitoring"
    condition: monitoring.enabled
  - name: network-policies
    version: "0.1.0"
    repository: "file://charts/network-policies"
    condition: network-policies.enabled
```

**ClusterIssuer template** (letsencrypt-prod 或 self-signed):
```yaml
{{ if .Values.clusterIssuer.letsencrypt.enabled }}
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: {{ .Values.clusterIssuer.letsencrypt.email | default "admin@example.com" }}
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
      - http01:
          ingress:
            class: {{ .Values.ingress.className | default "nginx" }}
{{ end }}

{{ if .Values.clusterIssuer.selfSigned.enabled }}
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
{{ end }}
```

**Certificate template** (自动证书):
```yaml
{{ if .Values.certificate.enabled }}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "cert-manager-issuer.fullname" . }}
spec:
  secretName: {{ .Values.certificate.secretName | default "devops-toolkit-tls" }}
  issuerRef:
    name: {{ .Values.certificate.issuerRef | default "selfsigned-issuer" }}
    kind: ClusterIssuer
  dnsNames:
    - {{ .Values.certificate.dnsName | default "devops.example.com" }}
{{ end }}
```

**values.yaml**:
```yaml
enabled: false
clusterIssuer:
  letsencrypt:
    enabled: false
    email: admin@example.com
  selfSigned:
    enabled: false
certificate:
  enabled: false
  secretName: devops-toolkit-tls
  issuerRef: selfsigned-issuer
  dnsName: devops.example.com
```

### 3.2 external-secrets 子 chart (Agent 2)

**Files:**
- `deploy/helm/charts/external-secrets/Chart.yaml`
- `deploy/helm/charts/external-secrets/values.yaml`
- `deploy/helm/charts/external-secrets/templates/secret-store.yaml`
- `deploy/helm/charts/external-secrets/templates/external-secret.yaml`
- `deploy/helm/charts/external-secrets/templates/_helpers.tpl`
- `deploy/helm/charts/external-secrets/README.md`

**SecretStore** (AWS Secrets Manager 示例):
```yaml
{{ if .Values.secretStore.enabled }}
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: {{ .Values.secretStore.name }}
spec:
  provider:
    aws:
      service: SecretsManager
      region: {{ .Values.secretStore.aws.region | default "us-east-1" }}
      auth:
        jwt:
          serviceAccountRef:
            name: {{ .Values.secretStore.aws.serviceAccountName | default "devops-toolkit" }}
{{ end }}
```

**ExternalSecret** (引用 SecretStore 拿真 secret):
```yaml
{{ if .Values.externalSecret.enabled }}
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: {{ include "external-secrets.fullname" . }}
spec:
  refreshInterval: {{ .Values.refreshInterval | default "1h" }}
  secretStoreRef:
    name: {{ .Values.secretStoreRef }}
    kind: SecretStore
  target:
    name: {{ .Values.target.name | default "devops-toolkit-secrets" }}
  data:
    {{- range .Values.data }}
    - secretKey: {{ .secretKey }}
      remoteRef:
        key: {{ .remoteRef.key }}
        property: {{ .remoteRef.property }}
    {{- end }}
{{ end }}
```

**values.yaml**:
```yaml
enabled: false
secretStore:
  enabled: false
  name: aws-secrets-manager
  aws:
    region: us-east-1
    serviceAccountName: devops-toolkit
externalSecret:
  enabled: false
  secretStoreRef: aws-secrets-manager
  target:
    name: devops-toolkit-secrets
  data:
    - secretKey: APP_JWT_SECRET
      remoteRef:
        key: devops/prod
        property: jwt_secret
    - secretKey: K8S_CRYPTO_KEY
      remoteRef:
        key: devops/prod
        property: k8s_crypto_key
refreshInterval: 1h
```

### 3.3 sealed-secrets 子 chart (Agent 3)

**Files:**
- `deploy/helm/charts/sealed-secrets/Chart.yaml`
- `deploy/helm/charts/sealed-secrets/values.yaml`
- `deploy/helm/charts/sealed-secrets/templates/sealed-secret.yaml`
- `deploy/helm/charts/sealed-secrets/templates/_helpers.tpl`
- `deploy/helm/charts/sealed-secrets/README.md`

**SealedSecret** (静态加密,免云依赖):
```yaml
{{ if .Values.enabled }}
apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: {{ .Values.name }}
  namespace: {{ .Values.namespace | default "default" }}
spec:
  encryptedData:
    {{- range $key, $value := .Values.encryptedData }}
    {{ $key }}: {{ $value }}
    {{- end }}
  template:
    metadata:
      name: {{ .Values.name }}
      namespace: {{ .Values.namespace | default "default" }}
{{ end }}
```

**values.yaml** (示例):
```yaml
enabled: false
name: devops-toolkit-sealed
namespace: default
# encryptedData 是用 `kubeseal` 工具预先加密的 base64 字符串
# 用户生成方式: kubeseal --format yaml < secret.yaml > sealed-secret.yaml
encryptedData:
  APP_JWT_SECRET: AgBxxxxxxxxxxxxxxxxx...  (placeholder)
  K8S_CRYPTO_KEY: AgByyyyyyyyyyyyyyyy...  (placeholder)
```

**注意**: 实际 .Values.encryptedData 应该是用 `kubeseal` 工具预先加密的 base64 字符串,用户生成。

### 3.4 monitoring 子 chart (Agent 4)

**Files:**
- `deploy/helm/charts/monitoring/Chart.yaml`
- `deploy/helm/charts/monitoring/values.yaml`
- `deploy/helm/charts/monitoring/templates/service-monitor.yaml`
- `deploy/helm/charts/monitoring/templates/pod-monitor.yaml`
- `deploy/helm/charts/monitoring/templates/_helpers.tpl`
- `deploy/helm/charts/monitoring/README.md`

**ServiceMonitor** (Prometheus 抓 Go `/metrics` endpoint):
```yaml
{{ if .Values.serviceMonitor.enabled }}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ include "monitoring.fullname" . }}
  labels:
    {{- include "monitoring.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "monitoring.selectorLabels" . | nindent 6 }}
  endpoints:
    - port: http
      path: /metrics
      interval: {{ .Values.serviceMonitor.interval | default "30s" }}
{{ end }}
```

**PodMonitor** (per-pod metrics):
```yaml
{{ if .Values.podMonitor.enabled }}
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: {{ include "monitoring.fullname" . }}-pods
  labels:
    {{- include "monitoring.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "monitoring.selectorLabels" . | nindent 6 }}
  podMetricsEndpoints:
    - port: http
      path: /metrics
      interval: {{ .Values.podMonitor.interval | default "30s" }}
{{ end }}
```

**values.yaml**:
```yaml
enabled: false
serviceMonitor:
  enabled: false
  interval: 30s
podMonitor:
  enabled: false
  interval: 30s
```

### 3.5 network-policies 子 chart (主 session 协调)

**Files:**
- `deploy/helm/charts/network-policies/Chart.yaml`
- `deploy/helm/charts/network-policies/values.yaml`
- `deploy/helm/charts/network-policies/templates/network-policy.yaml`
- `deploy/helm/charts/network-policies/templates/_helpers.tpl`
- `deploy/helm/charts/network-policies/README.md`

**NetworkPolicy** (默认 deny + explicit allow):
```yaml
{{ if .Values.enabled }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "network-policies.fullname" . }}
spec:
  podSelector:
    matchLabels:
      {{- include "network-policies.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Allow ingress from ingress-nginx in same namespace
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{ .Values.ingressNamespace | default "ingress-nginx" }}
      ports:
        - port: 8080
          protocol: TCP
  egress:
    # Allow DNS to kube-system
    - to:
        - namespaceSelector: {}
      ports:
        - port: 53
          protocol: UDP
    # Allow PostgreSQL (separate namespace)
    {{- if .Values.database.enabled }}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: {{ .Values.database.namespace | default "postgres" }}
      ports:
        - port: 5432
          protocol: TCP
    {{- end }}
{{ end }}
```

**values.yaml**:
```yaml
enabled: false
ingressNamespace: ingress-nginx
database:
  enabled: false
  namespace: postgres
```

### 3.6 minikube install 脚本 (主 session)

**File:** `scripts/install-minikube.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

# C-子项目: minikube 上 install 验证 DevOps Toolkit chart
# 用法: ./scripts/install-minikube.sh [--reset]

# Prerequisites check
which minikube || { echo "ERROR: minikube not installed"; exit 1; }
which helm || { echo "ERROR: helm not installed"; exit 1; }
which kubectl || { echo "ERROR: kubectl not installed"; exit 1; }

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

echo "==> Installing devops-toolkit chart"
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

echo "==> Done! Access at http://localhost:8080"
echo "==> Stop port-forward: kill $PORT_FORWARD_PID"
echo "==> minikube dashboard: minikube dashboard"
```

**File:** `deploy/helm/TESTING.md` (主 session 写):

```markdown
# DevOps Toolkit Helm Chart — Testing Guide

## Prerequisites
- minikube >= 1.30
- helm >= 3.12
- kubectl >= 1.27
- 4 CPU, 8GB RAM available

## Quick Start (minikube)
\`\`\`bash
./scripts/install-minikube.sh
\`\`\`

## Manual Test Steps
1. `minikube start --cpus=4 --memory=8g`
2. `cd deploy/helm && helm dep update`
3. `helm install devops . --set ingress.enabled=true --set ...` (see values.yaml for full options)
4. `kubectl wait --for=condition=available --timeout=300s deployment/devops-toolkit`
5. `kubectl port-forward svc/devops-toolkit 8080:80`
6. Open http://localhost:8080 in browser

## Verify each sub-chart
\`\`\`bash
# cert-manager-issuer
kubectl get clusterissuer
kubectl get certificate

# external-secrets
kubectl get secretstore
kubectl get externalsecret

# sealed-secrets
kubectl get sealedsecret

# monitoring
kubectl get servicemonitor
kubectl get podmonitor

# network-policies
kubectl get networkpolicy
\`\`\`

## Cleanup
\`\`\`bash
helm uninstall devops
minikube delete
\`\`\`
```

---

## 4. 数据流与错误处理

### 4.1 关键数据流

**Flow 1: 用户装 chart**
```
helm repo add devops ./deploy/helm  (umbrella)
helm install devops ./deploy/helm \
  --set cert-manager-issuer.enabled=true
  --set external-secrets.enabled=true
  --set monitoring.enabled=true
  --set network-policies.enabled=true
  ↓
helm dep update → 拉所有子 chart
  ↓
渲染主 chart templates (A 子项目 13 模板)
  ↓
渲染子 chart templates (5 子 chart)
  ↓
kubectl apply (CRDs 已装好)
  ↓
devops-toolkit deployment + ClusterIssuer + ExternalSecret + ServiceMonitor + NetworkPolicy
```

**Flow 2: cert-manager 签证书**
```
ClusterIssuer "letsencrypt-prod" 存在
  ↓
ingress 创建,annotation: cert-manager.io/cluster-issuer: letsencrypt-prod
  ↓
cert-manager controller 看到 ingress,自动创建 Certificate CRD
  ↓
Let's Encrypt HTTP-01 challenge
  ↓
Certificate.status.ready = true
  ↓
Secret "devops-toolkit-tls" 创建
  ↓
ingress 引用该 Secret,提供 TLS
```

**Flow 3: ExternalSecret 同步**
```
SecretStore "aws-secrets-manager" 存在
  ↓
ExternalSecret "devops-toolkit" 引用 SecretStore
  ↓
external-secrets operator (5 min interval) 调 AWS Secrets Manager
  ↓
拿到真 secret 数据
  ↓
自动创建/更新 Kubernetes Secret "devops-toolkit-secrets"
  ↓
devops-toolkit pod 启动时从该 Secret 读 APP_JWT_SECRET 等
```

**Flow 4: minikube 验证**
```
scripts/install-minikube.sh
  ↓
minikube start --cpus=4 --memory=8g
  ↓
helm dep update deploy/helm
  ↓
helm install devops ./deploy/helm --set ... (本机 values)
  ↓
kubectl wait deployment/devops-toolkit
  ↓
kubectl port-forward svc/devops-toolkit 8080:80
  ↓
curl http://localhost:8080/live  →  200
curl http://localhost:8080/ready →  200 (or 503 if DB unavailable)
```

### 4.2 错误处理约定

| 场景 | 行为 | log |
|---|---|---|
| helm dep update 失败(网络) | install 脚本退出,提示用户 | error |
| ClusterIssuer 还不存在,Cannot create cert | cert-manager 自动重试,用户看 status | info |
| ExternalSecret SecretStore 失败 | external-secrets 自动重试,status 提示 | warn |
| ServiceMonitor selector 不匹配 | Prometheus 0 scrape,alert 触发(后续 P5 配) | warn |
| NetworkPolicy 太严,pod 通信失败 | 调试模式自动打开,user inspect NetworkPolicy | warn |
| minikube 资源不足 | install 脚本退出,提示加 cpus/memory | error |

**关键约定**:
- **5 个子 chart 全部默认 disabled** — 用户 opt-in
- **错误永不 silent fail** — helm template 严格,任何缺失 required value 报错
- **NetworkPolicy 默认 deny** — fail-secure 模式
- **minikube install 失败 exit 1** — 不留 half-installed 状态

### 4.3 测试矩阵

| Component | Unit | Integration | E2E (manual) |
|---|---|---|---|
| cert-manager-issuer templates | ✅ helm template | ✅ helm install dry-run | ✅ minikube |
| external-secrets templates | ✅ helm template | ✅ helm install dry-run | ✅ minikube + AWS |
| sealed-secrets templates | ✅ helm template | ✅ helm install dry-run | ✅ minikube + sealed-secrets operator |
| monitoring templates | ✅ helm template | ✅ helm install dry-run | ✅ minikube + prometheus operator |
| network-policies templates | ✅ helm template | ✅ helm install dry-run | ✅ minikube + kubectl test connection |
| install-minikube.sh | ❌ (bash) | ❌ | ✅ manual on minikube |

**总测试新增**: ~15-20 helm template tests + 1-2 bash tests (shellcheck) + manual E2E
**CI**: helm template + helm lint 在 CI 跑;minikube 不在 CI 跑

### 4.4 风险 & 缓解

| 风险 | 缓解 |
|---|---|
| 5 agent 都改 deploy/helm/,并发 conflict | 各自改不同子 chart 目录,主 chart 改 Chart.yaml dependencies (避免撞) |
| 5 agent 改 Chart.yaml dependencies,顺序敏感 | 用 text-based merge,按字母顺序 (cert-manager, external, monitoring, network, sealed) |
| NetworkPolicy 太严,pod 调度失败 | 默认 disabled,user opt-in 后必须验证 |
| minikube 脚本不可在 CI 跑 | 仅文档化,手动验证,加 shellcheck |
| Chart.yaml dependencies 没自动 build | helm dep build 在 helm dep update 流程里,文档说明 |
| sub chart 模板语法错误 | helm template + helm lint 强制通过 |

---

## 5. 实施计划(5 phase,5 agent 并行)

### Phase 1-4: 4 个 agent 并行(子 chart)
- **Agent 1** (`feat/c1-cert-manager-issuer`): cert-manager ClusterIssuer + Certificate — 0.5 day
- **Agent 2** (`feat/c2-external-secrets`): ExternalSecret + SecretStore — 0.5 day
- **Agent 3** (`feat/c3-sealed-secrets`): SealedSecret 模板 — 0.5 day
- **Agent 4** (`feat/c4-monitoring`): ServiceMonitor + PodMonitor — 0.5 day

### Phase 5: Agent 5 + 主 session 协调
- **Agent 5** (`feat/c5-network-policies-install`): NetworkPolicy + minikube 脚本 + Chart.yaml dependencies 接线 + 文档 — 1-2 day
- 主 session: 收尾 merge + 验证 + 文档

### 时间轴
- Day 1: Agent 1-4 并行(4 子 chart)
- Day 2: Agent 5 收尾(NetworkPolicy + minikube 脚本 + 文档)
- Day 3: 主 session merge + 全量测试 + 收尾

### 关键 commit 命名(预计 ~15-20 commits)

**Phase 1 (Agent 1, ~2 commits)**
1. `feat(helm/cert-manager-issuer): add Chart.yaml + values.yaml + _helpers.tpl`
2. `feat(helm/cert-manager-issuer): add ClusterIssuer + Certificate templates + README`

**Phase 2 (Agent 2, ~2 commits)**
3. `feat(helm/external-secrets): add Chart.yaml + values.yaml + _helpers.tpl`
4. `feat(helm/external-secrets): add SecretStore + ExternalSecret templates + README`

**Phase 3 (Agent 3, ~2 commits)**
5. `feat(helm/sealed-secrets): add Chart.yaml + values.yaml + _helpers.tpl`
6. `feat(helm/sealed-secrets): add SealedSecret template + README`

**Phase 4 (Agent 4, ~2 commits)**
7. `feat(helm/monitoring): add Chart.yaml + values.yaml + _helpers.tpl`
8. `feat(helm/monitoring): add ServiceMonitor + PodMonitor templates + README`

**Phase 5 (Agent 5 + 主 session, ~8-10 commits)**
9. `feat(helm/network-policies): add Chart.yaml + values.yaml + _helpers.tpl`
10. `feat(helm/network-policies): add NetworkPolicy template + README`
11. `feat(helm): add 5 sub-chart dependencies to main Chart.yaml`
12. `feat(scripts): minikube install 验证脚本 (install-minikube.sh)`
13. `docs(helm): TESTING.md 详细 minikube 验证步骤`
14. `docs(helm): README.md 更新含 5 个子 chart 说明`
15. `chore(release): v0.5.0.0 — C 子项目 Helm 完整化文档收尾`

### 每个 commit 的完成定义 (DoD)

- 该 commit 的 templates/scripts 落地
- `helm template` 跑过(无错误)
- `helm lint` 跑过(无 warning/error)
- 既有 31 packages 全绿
- CHANGELOG.md 增一行
- commit message 用中文,符合 git log 风格

---

## 6. 风险与回退

### 风险

| 风险 | 缓解 |
|---|---|
| 5 agent 都改 deploy/helm/,并发 conflict | 各自改不同子 chart 目录,主 chart 改 Chart.yaml dependencies (避免撞) |
| 5 agent 改 Chart.yaml dependencies,顺序敏感 | 用 text-based merge,按字母顺序 (cert-manager, external, monitoring, network, sealed) |
| NetworkPolicy 太严,pod 调度失败 | 默认 disabled,user opt-in 后必须验证 |
| minikube 脚本不可在 CI 跑 | 仅文档化,手动验证,加 shellcheck |
| Chart.yaml dependencies 没自动 build | helm dep build 在 helm dep update 流程里,文档说明 |
| sub chart 模板语法错误 | helm template + helm lint 强制通过 |

### 回退(每个 phase 独立可回退)

| Phase | 回退方法 | 风险等级 |
|---|---|---|
| C1 cert-manager | `git revert <merge>` (1 PR,子 chart) | 🟢 低 |
| C2 external-secrets | `git revert` | 🟢 低 |
| C3 sealed-secrets | `git revert` | 🟢 低 |
| C4 monitoring | `git revert` | 🟢 低 |
| C5 NetworkPolicy + 脚本 | `git revert` | 🟡 中 — NetworkPolicy 启用后需立即 rollback |

---

## 7. 不做 (YAGNI)

- 不做 Velero(留后续)
- 不做 ArgoCD(留后续)
- 不做 Vault(留后续,只 ExternalSecret)
- 不做 OpenTelemetry operator(已有 tracing)
- 不做多 cluster / multi-region
- 不做 KEDA(高级 HPA,留后续)
- 不做 cert-manager operator 自身(用户自己装)
- 不做 sealed-secrets controller 自身(用户自己装)
- 不做 prometheus-operator 自身(用户自己装)
- 不做 network-policy-engine 复杂 eBPF(留后续)

---

## 8. 验收标准

### 完成定义 (Definition of Done)

C 子项目**完成**意味着:

1. **代码**: 5 个子 chart 全部就位 + 主 Chart.yaml dependencies 接线 + minikube 脚本 + TESTING.md + README
2. **测试**: `helm template` + `helm lint` 全过(子 chart 单独 + umbrella);`shellcheck scripts/install-minikube.sh` 通过
3. **质量**: 既有 31 packages 全绿,1 vet warning 既有
4. **文档**: CHANGELOG `[0.5.0.0]` section,DOCUMENT_INDEX 加 spec 索引,TESTING.md + README 更新
5. **E2E**: minikube install 跑过,/live 和 /ready 返 200(or 503 if DB),5 个子 chart 资源都创建

### 关键验收用例

| 验收项 | 验证 |
|---|---|
| helm template 全过 | `cd deploy/helm && helm dep update && helm template devops .` |
| helm lint 全过 | `helm lint deploy/helm/charts/<sub>` × 5 |
| minikube install | `./scripts/install-minikube.sh` 跑过 |
| /live 200 | `curl http://localhost:8080/live` 返 200 |
| /ready 200 or 503 | `curl http://localhost:8080/ready` 返非 401 |
| cert-manager 资源存在 | `kubectl get clusterissuer` 返 1+ |
| external-secrets 资源存在 | `kubectl get secretstore` 返 1+ |
| monitoring 资源存在 | `kubectl get servicemonitor` 返 1+ |
| network-policies 资源存在 | `kubectl get networkpolicy` 返 1+ |

---

## 9. 未来子项目 Preview

C 完成后(预计 1 周):

**E 子项目** (1 周): 性能 + 容量基线
- k6 100/300/1000 VU 场景
- p95+p99+error rate 报告
- Go pprof 内存/协程 profile
- 31 packages + 17+ vitest 全绿的性能基线

C 后续增强(留后续子项目):
- ArgoCD ApplicationSet (multi-cluster GitOps)
- Velero 备份跨 K8s cluster
- Vault 集成(替换 ExternalSecret)
- OpenTelemetry operator 完整 trace
- KEDA 高级 HPA
- eBPF-based NetworkPolicy engine

C→E 总估时 2 周,完成 v0.4.0.0 → v0.5.0.0。

---

## 附录 A:文件清单(详细变更范围)

### 新增文件
```
deploy/helm/charts/cert-manager-issuer/Chart.yaml
deploy/helm/charts/cert-manager-issuer/values.yaml
deploy/helm/charts/cert-manager-issuer/templates/_helpers.tpl
deploy/helm/charts/cert-manager-issuer/templates/cluster-issuer.yaml
deploy/helm/charts/cert-manager-issuer/templates/certificate.yaml
deploy/helm/charts/cert-manager-issuer/README.md

deploy/helm/charts/external-secrets/Chart.yaml
deploy/helm/charts/external-secrets/values.yaml
deploy/helm/charts/external-secrets/templates/_helpers.tpl
deploy/helm/charts/external-secrets/templates/secret-store.yaml
deploy/helm/charts/external-secrets/templates/external-secret.yaml
deploy/helm/charts/external-secrets/README.md

deploy/helm/charts/sealed-secrets/Chart.yaml
deploy/helm/charts/sealed-secrets/values.yaml
deploy/helm/charts/sealed-secrets/templates/_helpers.tpl
deploy/helm/charts/sealed-secrets/templates/sealed-secret.yaml
deploy/helm/charts/sealed-secrets/README.md

deploy/helm/charts/monitoring/Chart.yaml
deploy/helm/charts/monitoring/values.yaml
deploy/helm/charts/monitoring/templates/_helpers.tpl
deploy/helm/charts/monitoring/templates/service-monitor.yaml
deploy/helm/charts/monitoring/templates/pod-monitor.yaml
deploy/helm/charts/monitoring/README.md

deploy/helm/charts/network-policies/Chart.yaml
deploy/helm/charts/network-policies/values.yaml
deploy/helm/charts/network-policies/templates/_helpers.tpl
deploy/helm/charts/network-policies/templates/network-policy.yaml
deploy/helm/charts/network-policies/README.md

deploy/helm/TESTING.md
scripts/install-minikube.sh
```

### 修改文件
```
deploy/helm/Chart.yaml        (加 5 个 dependencies)
deploy/helm/README.md         (更新,加 5 个子 chart 说明)
```

### 跨 phase 依赖

```
C.Phase1-4 (4 agent)  ──┐
                        ├──→ C.Phase5 (Agent 5: NetworkPolicy + minikube 脚本 + 收尾)
                        │
C.Phase5 改主 Chart.yaml dependencies (在子 chart 落地后)
```

5 phase 可并行,Phase 5 收尾依赖主 Chart.yaml 引用子 chart。
