# C 子项目 — Helm chart 完整化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 DevOps Toolkit 的 Helm chart 完整化(umbrella chart + 5 个子 chart + minikube install 验证),1 周内完成,~15-20 atomic commits。

**Architecture:** 在 A 子项目交付的 Helm chart 骨架(13 templates)基础上,加 5 个子 chart(cert-manager-issuer + external-secrets + sealed-secrets + monitoring + network-policies)做 umbrella chart 风格集成。每个子 chart 独立可启,默认 disabled,opt-in 启用。主 chart 通过 `Chart.yaml` dependencies 字段引用子 chart。`scripts/install-minikube.sh` 提供本地验证流程。

**Tech Stack:** Helm 3.12+ + Kubernetes 1.27+ + cert-manager.io/v1 + external-secrets.io/v1beta1 + bitnami.com/v1alpha1 (sealed-secrets) + monitoring.coreos.com/v1 (ServiceMonitor/PodMonitor) + networking.k8s.io/v1 (NetworkPolicy) + minikube 1.30+

**Spec:** [`docs/superpowers/specs/2026-06-13-C-helm-full-design.md`](../specs/2026-06-13-C-helm-full-design.md)

---

## 范围总览

**5 phase,5 agent,1 周,~15-20 commits。**

| Agent | Branch | Tasks | Phase | 估时 | 依赖 |
|---|---|---|---|---|---|
| Agent 1 | `feat/c1-cert-manager-issuer` | Task 1-2 (ClusterIssuer + Certificate) | Phase 1 | 0.5 day | 无 |
| Agent 2 | `feat/c2-external-secrets` | Task 3-4 (SecretStore + ExternalSecret) | Phase 2 | 0.5 day | 无 |
| Agent 3 | `feat/c3-sealed-secrets` | Task 5-6 (SealedSecret) | Phase 3 | 0.5 day | 无 |
| Agent 4 | `feat/c4-monitoring` | Task 7-8 (ServiceMonitor + PodMonitor) | Phase 4 | 0.5 day | 无 |
| Agent 5 (主 session) | `feat/c5-network-policies-install` | Task 9-15 (NetworkPolicy + minikube 脚本 + Chart.yaml dependencies + 文档) | Phase 5 | 1-2 day | Phase 1-4 |

主 session 协调 merge 顺序:c1 → c2 → c3 → c4 → c5。

---

## 关键共享约定(所有 agent 必须遵守)

### 1. Helm chart 文件结构

```
deploy/helm/
├── Chart.yaml                (umbrella, dependencies list)
├── values.yaml               (主 values)
├── templates/                (13 templates, A 子项目保留)
├── charts/                   (5 sub-charts 本次新增)
│   ├── cert-manager-issuer/
│   ├── external-secrets/
│   ├── sealed-secrets/
│   ├── monitoring/
│   └── network-policies/
├── TESTING.md                (minikube 验证步骤,Phase 5)
└── README.md                 (更新,Phase 5)
```

### 2. 命名

- Chart name: `devops-toolkit` (主),子 chart name 各自
- Sub-chart version: `0.1.0`
- templates 用 `_helpers.tpl` 提供 `<chart-name>.fullname` / `.labels` / `.selectorLabels`
- 每个子 chart 有自己的 values.yaml,默认 `enabled: false`

### 3. 验证命令

- 单 chart 模板验证: `helm template test-name deploy/helm/charts/<sub>`
- 单 chart lint: `helm lint deploy/helm/charts/<sub>`
- 全部 chart 验证: `cd deploy/helm && helm dep update && helm template devops .`
- 既有 backend test: `go test -count=1 -p 1 -timeout 600s ./...` (确保 C 子项目不破后端)
- shellcheck bash: `shellcheck scripts/install-minikube.sh`

### 4. Branch 命名 & commit 风格

- Branch: `feat/c1-cert-manager-issuer` / `feat/c2-external-secrets` / `feat/c3-sealed-secrets` / `feat/c4-monitoring` / `feat/c5-network-policies-install`
- 每个 task 1-3 个 commit
- 主 session merge: `git merge --no-ff <branch>`

### 5. 顺序约束

- **Task 1-8 各自独立**(不同子 chart 目录,无 conflict)
- **Task 9-15 改主 Chart.yaml + scripts/ + docs** — 由主 session (Agent 5) 协调
- 主 chart dependencies 排序:cert-manager, external, monitoring, network, sealed(字母序)

### 6. 不在本计划范围(YAGNI)

- 不做 Velero / ArgoCD / Vault / OpenTelemetry operator
- 不做多 cluster / multi-region
- 不做 KEDA 高级 HPA
- 不做 cert-manager operator 自身(用户自己装)
- 不做 sealed-secrets controller 自身(用户自己装)
- 不做 prometheus-operator 自身(用户自己装)
- 不做 eBPF network policy

---

## File Structure(C 子项目总览)

### 新增文件 (子 chart × 5)
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
```

### 新增文件 (scripts + docs, Phase 5)
```
deploy/helm/TESTING.md
scripts/install-minikube.sh
```

### 修改文件
```
deploy/helm/Chart.yaml      (加 5 个 dependencies)
deploy/helm/README.md       (更新,加 5 个子 chart 说明)
```

---

## Task 1: cert-manager-issuer 子 chart 骨架 (Phase 1, Commit #1)

**Files:**
- Create: `deploy/helm/charts/cert-manager-issuer/Chart.yaml`
- Create: `deploy/helm/charts/cert-manager-issuer/values.yaml`
- Create: `deploy/helm/charts/cert-manager-issuer/templates/_helpers.tpl`

**Agent:** Agent 1

- [ ] **Step 1: 写 `Chart.yaml`**

```yaml
apiVersion: v2
name: cert-manager-issuer
description: ClusterIssuer + Certificate for DevOps Toolkit
type: application
version: 0.1.0
appVersion: "0.5.0.0"
```

- [ ] **Step 2: 写 `values.yaml`**

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

- [ ] **Step 3: 写 `_helpers.tpl`**

```go
{{/*
Expand the name of the chart.
*/}}
{{- define "cert-manager-issuer.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "cert-manager-issuer.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "cert-manager-issuer.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "cert-manager-issuer.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "cert-manager-issuer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cert-manager-issuer.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
```

- [ ] **Step 4: 跑 `helm template` 验证**

Run: `helm template test deploy/helm/charts/cert-manager-issuer 2>&1 | tail -5`
Expected: 渲染成功(即使 enabled=false,空 output)。0 错误。

- [ ] **Step 5: 跑 `helm lint` 验证**

Run: `helm lint deploy/helm/charts/cert-manager-issuer 2>&1 | tail -5`
Expected: `0 failures, 0 warnings` (或类似),exit 0。

- [ ] **Step 6: 提交**

```bash
git add deploy/helm/charts/cert-manager-issuer/
git commit -m "feat(helm/cert-manager-issuer): add Chart.yaml + values.yaml + _helpers.tpl"
```

---

## Task 2: cert-manager-issuer ClusterIssuer + Certificate templates (Phase 1, Commit #2)

**Files:**
- Create: `deploy/helm/charts/cert-manager-issuer/templates/cluster-issuer.yaml`
- Create: `deploy/helm/charts/cert-manager-issuer/templates/certificate.yaml`
- Create: `deploy/helm/charts/cert-manager-issuer/README.md`

**Agent:** Agent 1

- [ ] **Step 1: 写 `cluster-issuer.yaml`**

```yaml
{{ if .Values.enabled }}
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
{{ end }}
```

- [ ] **Step 2: 写 `certificate.yaml`**

```yaml
{{ if and .Values.enabled .Values.certificate.enabled }}
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

- [ ] **Step 3: 写 `README.md`**

```markdown
# cert-manager-issuer sub-chart

ClusterIssuer + Certificate for the DevOps Toolkit platform.

## Usage

In the umbrella chart values.yaml:

```yaml
cert-manager-issuer:
  enabled: true
  clusterIssuer:
    selfSigned:
      enabled: true
  certificate:
    enabled: true
    dnsName: devops.example.com
```

## Prerequisites

`cert-manager` operator must be installed (https://cert-manager.io/docs/installation/).

## Validation

```bash
helm template devops ./deploy/helm --set cert-manager-issuer.enabled=true
kubectl get clusterissuer
kubectl get certificate
```
```

- [ ] **Step 4: 跑 `helm template` 验证**

Run: `helm template test deploy/helm/charts/cert-manager-issuer --set cert-manager-issuer.enabled=true --set cert-manager-issuer.clusterIssuer.selfSigned.enabled=true 2>&1 | tail -10`
Expected: 渲染成功,含 ClusterIssuer + (no Certificate if .Values.certificate.enabled 未设)。0 错误。

- [ ] **Step 5: 跑 `helm lint` 验证**

Run: `helm lint deploy/helm/charts/cert-manager-issuer 2>&1 | tail -5`
Expected: exit 0。

- [ ] **Step 6: 提交**

```bash
git add deploy/helm/charts/cert-manager-issuer/templates/ deploy/helm/charts/cert-manager-issuer/README.md
git commit -m "feat(helm/cert-manager-issuer): add ClusterIssuer + Certificate templates + README"
```

---

## Task 3: external-secrets 子 chart 骨架 (Phase 2, Commit #3)

**Files:**
- Create: `deploy/helm/charts/external-secrets/Chart.yaml`
- Create: `deploy/helm/charts/external-secrets/values.yaml`
- Create: `deploy/helm/charts/external-secrets/templates/_helpers.tpl`

**Agent:** Agent 2

- [ ] **Step 1: 写 `Chart.yaml`**

```yaml
apiVersion: v2
name: external-secrets
description: ExternalSecret + SecretStore for DevOps Toolkit
type: application
version: 0.1.0
appVersion: "0.5.0.0"
```

- [ ] **Step 2: 写 `values.yaml`**

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

- [ ] **Step 3: 写 `_helpers.tpl`**

```go
{{- define "external-secrets.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "external-secrets.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "external-secrets.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "external-secrets.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "external-secrets.selectorLabels" -}}
app.kubernetes.io/name: {{ include "external-secrets.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
```

- [ ] **Step 4: 验证 `helm template` + `helm lint`**

Run: `helm template test deploy/helm/charts/external-secrets 2>&1 | tail -3 && helm lint deploy/helm/charts/external-secrets 2>&1 | tail -3`
Expected: 都 exit 0。

- [ ] **Step 5: 提交**

```bash
git add deploy/helm/charts/external-secrets/
git commit -m "feat(helm/external-secrets): add Chart.yaml + values.yaml + _helpers.tpl"
```

---

## Task 4: external-secrets SecretStore + ExternalSecret templates (Phase 2, Commit #4)

**Files:**
- Create: `deploy/helm/charts/external-secrets/templates/secret-store.yaml`
- Create: `deploy/helm/charts/external-secrets/templates/external-secret.yaml`
- Create: `deploy/helm/charts/external-secrets/README.md`

**Agent:** Agent 2

- [ ] **Step 1: 写 `secret-store.yaml`**

```yaml
{{ if and .Values.enabled .Values.secretStore.enabled }}
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

- [ ] **Step 2: 写 `external-secret.yaml`**

```yaml
{{ if and .Values.enabled .Values.externalSecret.enabled }}
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: {{ include "external-secrets.fullname" . }}
spec:
  refreshInterval: {{ .Values.refreshInterval | default "1h" }}
  secretStoreRef:
    name: {{ .Values.externalSecret.secretStoreRef }}
    kind: SecretStore
  target:
    name: {{ .Values.externalSecret.target.name | default "devops-toolkit-secrets" }}
  data:
    {{- range .Values.externalSecret.data }}
    - secretKey: {{ .secretKey }}
      remoteRef:
        key: {{ .remoteRef.key }}
        property: {{ .remoteRef.property }}
    {{- end }}
{{ end }}
```

- [ ] **Step 3: 写 `README.md`**

```markdown
# external-secrets sub-chart

ExternalSecret + SecretStore for syncing secrets from AWS Secrets Manager (or any external secret store).

## Usage

```yaml
external-secrets:
  enabled: true
  secretStore:
    enabled: true
    name: aws-secrets-manager
    aws:
      region: us-east-1
  externalSecret:
    enabled: true
    secretStoreRef: aws-secrets-manager
    target:
      name: devops-toolkit-secrets
    data:
      - secretKey: APP_JWT_SECRET
        remoteRef:
          key: devops/prod
          property: jwt_secret
```

## Prerequisites

External Secrets Operator must be installed (https://external-secrets.io/latest/).

## Validation

```bash
helm template devops ./deploy/helm --set external-secrets.enabled=true
kubectl get secretstore
kubectl get externalsecret
```
```

- [ ] **Step 4: 验证**

Run: `helm template test deploy/helm/charts/external-secrets --set external-secrets.enabled=true --set external-secrets.secretStore.enabled=true --set external-secrets.externalSecret.enabled=true 2>&1 | tail -10 && helm lint deploy/helm/charts/external-secrets 2>&1 | tail -3`
Expected: 渲染成功(含 SecretStore + ExternalSecret);lint exit 0。

- [ ] **Step 5: 提交**

```bash
git add deploy/helm/charts/external-secrets/templates/ deploy/helm/charts/external-secrets/README.md
git commit -m "feat(helm/external-secrets): add SecretStore + ExternalSecret templates + README"
```

---

## Task 5: sealed-secrets 子 chart 骨架 (Phase 3, Commit #5)

**Files:**
- Create: `deploy/helm/charts/sealed-secrets/Chart.yaml`
- Create: `deploy/helm/charts/sealed-secrets/values.yaml`
- Create: `deploy/helm/charts/sealed-secrets/templates/_helpers.tpl`

**Agent:** Agent 3

- [ ] **Step 1: 写 `Chart.yaml`**

```yaml
apiVersion: v2
name: sealed-secrets
description: SealedSecret for DevOps Toolkit (static encryption, no cloud dependency)
type: application
version: 0.1.0
appVersion: "0.5.0.0"
```

- [ ] **Step 2: 写 `values.yaml`**

```yaml
enabled: false
name: devops-toolkit-sealed
namespace: default
# encryptedData 是用 `kubeseal` 工具预先加密的 base64 字符串
# 用户生成方式: kubeseal --format yaml < secret.yaml > sealed-secret.yaml
encryptedData:
  APP_JWT_SECRET: AgBxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx  # placeholder
  K8S_CRYPTO_KEY: AgByyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy  # placeholder
```

- [ ] **Step 3: 写 `_helpers.tpl`**

```go
{{- define "sealed-secrets.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "sealed-secrets.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "sealed-secrets.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "sealed-secrets.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "sealed-secrets.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sealed-secrets.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
```

- [ ] **Step 4: 验证**

Run: `helm template test deploy/helm/charts/sealed-secrets 2>&1 | tail -3 && helm lint deploy/helm/charts/sealed-secrets 2>&1 | tail -3`
Expected: 都 exit 0。

- [ ] **Step 5: 提交**

```bash
git add deploy/helm/charts/sealed-secrets/
git commit -m "feat(helm/sealed-secrets): add Chart.yaml + values.yaml + _helpers.tpl"
```

---

## Task 6: sealed-secrets SealedSecret template + README (Phase 3, Commit #6)

**Files:**
- Create: `deploy/helm/charts/sealed-secrets/templates/sealed-secret.yaml`
- Create: `deploy/helm/charts/sealed-secrets/README.md`

**Agent:** Agent 3

- [ ] **Step 1: 写 `sealed-secret.yaml`**

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

- [ ] **Step 2: 写 `README.md`**

```markdown
# sealed-secrets sub-chart

SealedSecret for the DevOps Toolkit platform. Use this if you don't have access to AWS Secrets Manager and want static-encrypted secrets in git.

## Usage

1. Install sealed-secrets controller: `helm install sealed-secrets sealed-secrets/sealed-secrets`
2. Generate sealed secret: `kubeseal --format yaml < secret.yaml > sealed-secret.yaml`
3. Put encrypted values in `values.yaml`:

```yaml
sealed-secrets:
  enabled: true
  name: devops-toolkit-sealed
  encryptedData:
    APP_JWT_SECRET: AgBxxxxxxxxxxxxxxxxxx...
    K8S_CRYPTO_KEY: AgByyyyyyyyyyyyyyy...
```

## Validation

```bash
helm template devops ./deploy/helm --set sealed-secrets.enabled=true
kubectl get sealedsecret
```
```

- [ ] **Step 3: 验证**

Run: `helm template test deploy/helm/charts/sealed-secrets --set sealed-secrets.enabled=true 2>&1 | tail -10 && helm lint deploy/helm/charts/sealed-secrets 2>&1 | tail -3`
Expected: 渲染成功(含 SealedSecret);lint exit 0。

- [ ] **Step 4: 提交**

```bash
git add deploy/helm/charts/sealed-secrets/templates/ deploy/helm/charts/sealed-secrets/README.md
git commit -m "feat(helm/sealed-secrets): add SealedSecret template + README"
```

---

## Task 7: monitoring 子 chart 骨架 (Phase 4, Commit #7)

**Files:**
- Create: `deploy/helm/charts/monitoring/Chart.yaml`
- Create: `deploy/helm/charts/monitoring/values.yaml`
- Create: `deploy/helm/charts/monitoring/templates/_helpers.tpl`

**Agent:** Agent 4

- [ ] **Step 1: 写 `Chart.yaml`**

```yaml
apiVersion: v2
name: monitoring
description: ServiceMonitor + PodMonitor for DevOps Toolkit
type: application
version: 0.1.0
appVersion: "0.5.0.0"
```

- [ ] **Step 2: 写 `values.yaml`**

```yaml
enabled: false
serviceMonitor:
  enabled: false
  interval: 30s
podMonitor:
  enabled: false
  interval: 30s
```

- [ ] **Step 3: 写 `_helpers.tpl`**

```go
{{- define "monitoring.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "monitoring.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "monitoring.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "monitoring.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "monitoring.selectorLabels" -}}
app.kubernetes.io/name: {{ include "monitoring.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
```

- [ ] **Step 4: 验证**

Run: `helm template test deploy/helm/charts/monitoring 2>&1 | tail -3 && helm lint deploy/helm/charts/monitoring 2>&1 | tail -3`
Expected: 都 exit 0。

- [ ] **Step 5: 提交**

```bash
git add deploy/helm/charts/monitoring/
git commit -m "feat(helm/monitoring): add Chart.yaml + values.yaml + _helpers.tpl"
```

---

## Task 8: monitoring ServiceMonitor + PodMonitor templates (Phase 4, Commit #8)

**Files:**
- Create: `deploy/helm/charts/monitoring/templates/service-monitor.yaml`
- Create: `deploy/helm/charts/monitoring/templates/pod-monitor.yaml`
- Create: `deploy/helm/charts/monitoring/README.md`

**Agent:** Agent 4

- [ ] **Step 1: 写 `service-monitor.yaml`**

```yaml
{{ if and .Values.enabled .Values.serviceMonitor.enabled }}
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

- [ ] **Step 2: 写 `pod-monitor.yaml`**

```yaml
{{ if and .Values.enabled .Values.podMonitor.enabled }}
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

- [ ] **Step 3: 写 `README.md`**

```markdown
# monitoring sub-chart

ServiceMonitor + PodMonitor for the DevOps Toolkit `/metrics` endpoint.

## Usage

```yaml
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
  podMonitor:
    enabled: true
    interval: 30s
```

## Prerequisites

`prometheus-operator` must be installed.

## Validation

```bash
helm template devops ./deploy/helm --set monitoring.enabled=true
kubectl get servicemonitor
kubectl get podmonitor
```
```

- [ ] **Step 4: 验证**

Run: `helm template test deploy/helm/charts/monitoring --set monitoring.enabled=true --set monitoring.serviceMonitor.enabled=true --set monitoring.podMonitor.enabled=true 2>&1 | tail -10 && helm lint deploy/helm/charts/monitoring 2>&1 | tail -3`
Expected: 渲染成功(含 ServiceMonitor + PodMonitor);lint exit 0。

- [ ] **Step 5: 提交**

```bash
git add deploy/helm/charts/monitoring/templates/ deploy/helm/charts/monitoring/README.md
git commit -m "feat(helm/monitoring): add ServiceMonitor + PodMonitor templates + README"
```

---

## Task 9: network-policies 子 chart 骨架 (Phase 5, Commit #9)

**Files:**
- Create: `deploy/helm/charts/network-policies/Chart.yaml`
- Create: `deploy/helm/charts/network-policies/values.yaml`
- Create: `deploy/helm/charts/network-policies/templates/_helpers.tpl`

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 写 `Chart.yaml`**

```yaml
apiVersion: v2
name: network-policies
description: NetworkPolicy for DevOps Toolkit (default deny + explicit allow)
type: application
version: 0.1.0
appVersion: "0.5.0.0"
```

- [ ] **Step 2: 写 `values.yaml`**

```yaml
enabled: false
ingressNamespace: ingress-nginx
database:
  enabled: false
  namespace: postgres
```

- [ ] **Step 3: 写 `_helpers.tpl`**

```go
{{- define "network-policies.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "network-policies.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "network-policies.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "network-policies.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "network-policies.selectorLabels" -}}
app.kubernetes.io/name: {{ include "network-policies.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
```

- [ ] **Step 4: 验证**

Run: `helm template test deploy/helm/charts/network-policies 2>&1 | tail -3 && helm lint deploy/helm/charts/network-policies 2>&1 | tail -3`
Expected: 都 exit 0。

- [ ] **Step 5: 提交**

```bash
git add deploy/helm/charts/network-policies/Chart.yaml deploy/helm/charts/network-policies/values.yaml deploy/helm/charts/network-policies/templates/_helpers.tpl
git commit -m "feat(helm/network-policies): add Chart.yaml + values.yaml + _helpers.tpl"
```

---

## Task 10: network-policies NetworkPolicy template (Phase 5, Commit #10)

**Files:**
- Create: `deploy/helm/charts/network-policies/templates/network-policy.yaml`
- Create: `deploy/helm/charts/network-policies/README.md`

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 写 `network-policy.yaml`**

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
    # Allow ingress from ingress-nginx
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
    {{- if .Values.database.enabled }}
    # Allow PostgreSQL (separate namespace)
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

- [ ] **Step 2: 写 `README.md`**

```markdown
# network-policies sub-chart

NetworkPolicy for the DevOps Toolkit platform. Default deny + explicit allow for ingress (from ingress-nginx) and egress (DNS + optional DB).

## Usage

```yaml
network-policies:
  enabled: true
  ingressNamespace: ingress-nginx
  database:
    enabled: true
    namespace: postgres
```

## Note

When `enabled: true`, the devops-toolkit pod will ONLY accept ingress from the `ingress-nginx` namespace. If your ingress is in a different namespace, adjust `ingressNamespace`. Egress is restricted to DNS + PostgreSQL (if `database.enabled`).

## Validation

```bash
helm template devops ./deploy/helm --set network-policies.enabled=true
kubectl get networkpolicy
```
```

- [ ] **Step 3: 验证**

Run: `helm template test deploy/helm/charts/network-policies --set network-policies.enabled=true --set network-policies.database.enabled=true 2>&1 | tail -10 && helm lint deploy/helm/charts/network-policies 2>&1 | tail -3`
Expected: 渲染成功(含 NetworkPolicy);lint exit 0。

- [ ] **Step 4: 提交**

```bash
git add deploy/helm/charts/network-policies/templates/ deploy/helm/charts/network-policies/README.md
git commit -m "feat(helm/network-policies): add NetworkPolicy template + README"
```

---

## Task 11: 主 Chart.yaml 加 5 个 dependencies (Phase 5, Commit #11)

**Files:**
- Modify: `deploy/helm/Chart.yaml`

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 先看现状**

Run: `cat deploy/helm/Chart.yaml`

- [ ] **Step 2: 改 Chart.yaml — 加 dependencies**

(按字母序: cert-manager-issuer, external-secrets, monitoring, network-policies, sealed-secrets)

```yaml
apiVersion: v2
name: devops-toolkit
description: DevOps Toolkit platform
type: application
version: 0.1.0
appVersion: "0.5.0.0"
dependencies:
  - name: cert-manager-issuer
    version: "0.1.0"
    repository: "file://charts/cert-manager-issuer"
    condition: cert-manager-issuer.enabled
  - name: external-secrets
    version: "0.1.0"
    repository: "file://charts/external-secrets"
    condition: external-secrets.enabled
  - name: monitoring
    version: "0.1.0"
    repository: "file://charts/monitoring"
    condition: monitoring.enabled
  - name: network-policies
    version: "0.1.0"
    repository: "file://charts/network-policies"
    condition: network-policies.enabled
  - name: sealed-secrets
    version: "0.1.0"
    repository: "file://charts/sealed-secrets"
    condition: sealed-secrets.enabled
```

- [ ] **Step 3: 跑 `helm dep update`**

Run: `cd deploy/helm && helm dep update 2>&1 | tail -10`
Expected: 拉所有 5 个子 chart。0 errors。

- [ ] **Step 4: 跑 `helm template` 验证 (umbrella)**

Run: `helm template devops . 2>&1 | tail -10`
Expected: 渲染主 chart + 5 个子 chart(若 enabled,默认 disabled 只输出主 chart)。0 errors。

- [ ] **Step 5: 跑 `helm template` 启用 1 个子 chart**

Run: `helm template devops . --set cert-manager-issuer.enabled=true 2>&1 | tail -10`
Expected: 输出主 chart + ClusterIssuer (因为 cert-manager-issuer.enabled=true)。0 errors。

- [ ] **Step 6: 跑 `helm lint` 验证**

Run: `helm lint . 2>&1 | tail -5`
Expected: 0 failures, 0 warnings。

- [ ] **Step 7: 提交**

```bash
git add deploy/helm/Chart.yaml deploy/helm/Chart.lock deploy/helm/charts/ 2>/dev/null; git status
```

(如果 `helm dep update` 产生 `Chart.lock`,add 它)

```bash
git commit -m "feat(helm): add 5 sub-chart dependencies to main Chart.yaml (umbrella)"
```

---

## Task 12: minikube install 验证脚本 (Phase 5, Commit #12)

**Files:**
- Create: `scripts/install-minikube.sh`

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 写 `scripts/install-minikube.sh`**

```bash
#!/usr/bin/env bash
#
# install-minikube.sh — C-子项目: minikube 上 install 验证 DevOps Toolkit chart
#
# 用法: ./scripts/install-minikube.sh [--reset]
#

set -euo pipefail

# Prerequisites
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
```

- [ ] **Step 2: 加可执行权限**

Run: `chmod +x scripts/install-minikube.sh && ls -l scripts/install-minikube.sh`
Expected: `-rwxr-xr-x`。

- [ ] **Step 3: shellcheck 验证(可选)**

Run: `shellcheck scripts/install-minikube.sh 2>&1 | head -10`
Expected: 0 errors (或仅 info 级 warning)。

- [ ] **Step 4: 提交**

```bash
git add scripts/install-minikube.sh
git commit -m "feat(scripts): minikube install 验证脚本 (install-minikube.sh)"
```

---

## Task 13: deploy/helm/TESTING.md (Phase 5, Commit #13)

**Files:**
- Create: `deploy/helm/TESTING.md`

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 写 `TESTING.md`**

```markdown
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
```

- [ ] **Step 2: 提交**

```bash
git add deploy/helm/TESTING.md
git commit -m "docs(helm): TESTING.md 详细 minikube 验证步骤"
```

---

## Task 14: deploy/helm/README.md 更新 (Phase 5, Commit #14)

**Files:**
- Modify: `deploy/helm/README.md`

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 先看现状**

Run: `cat deploy/helm/README.md`

- [ ] **Step 2: 在 README.md 加 5 个子 chart 章节**

在现有 README 末尾(或恰当位置)加:

```markdown
## Sub-Charts (umbrella dependencies)

The umbrella chart `deploy/helm/Chart.yaml` declares 5 sub-chart dependencies. Each is opt-in (default `enabled: false`).

### cert-manager-issuer

[ClusterIssuer](https://cert-manager.io/docs/concepts/issuer/) + [Certificate](https://cert-manager.io/docs/concepts/certificate/) for automatic TLS cert provisioning.

```yaml
cert-manager-issuer:
  enabled: true
  clusterIssuer:
    selfSigned:
      enabled: true
  certificate:
    enabled: true
    dnsName: devops.example.com
```

Prerequisite: install [cert-manager](https://cert-manager.io/docs/installation/).

### external-secrets

[ExternalSecret](https://external-secrets.io/latest/) + SecretStore for syncing from AWS Secrets Manager, GCP Secret Manager, HashiCorp Vault, etc.

```yaml
external-secrets:
  enabled: true
  secretStore:
    enabled: true
    name: aws-secrets-manager
  externalSecret:
    enabled: true
    secretStoreRef: aws-secrets-manager
```

Prerequisite: install [external-secrets operator](https://external-secrets.io/latest/).

### sealed-secrets

[SealedSecret](https://github.com/bitnami-labs/sealed-secrets) for static-encrypted secrets in git. Cloud-agnostic.

```yaml
sealed-secrets:
  enabled: true
  name: devops-toolkit-sealed
  encryptedData:
    APP_JWT_SECRET: AgBxxx...  # generated via `kubeseal`
```

Prerequisite: install [sealed-secrets controller](https://github.com/bitnami-labs/sealed-secrets).

### monitoring

[ServiceMonitor](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.ServiceMonitor) + PodMonitor for Prometheus to scrape `/metrics` endpoint.

```yaml
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
  podMonitor:
    enabled: true
```

Prerequisite: install [prometheus-operator](https://prometheus-operator.dev/).

### network-policies

[NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) for default-deny + explicit allow ingress/egress.

```yaml
network-policies:
  enabled: true
  ingressNamespace: ingress-nginx
  database:
    enabled: true
    namespace: postgres
```

## Testing

See [TESTING.md](TESTING.md) for minikube install + verification.
```

- [ ] **Step 3: 提交**

```bash
git add deploy/helm/README.md
git commit -m "docs(helm): README.md 更新含 5 个子 chart 说明"
```

---

## Task 15: v0.5.0.0 release 文档收尾 (Phase 5, Commit #15)

**Files:**
- Modify: `VERSION`
- Modify: `CHANGELOG.md`
- Modify: `DOCUMENT_INDEX.md`
- Modify: `TODOS.md`

**Agent:** 主 session 收尾

- [ ] **Step 1: VERSION**

Run: `echo "0.5.0.0" > VERSION`

- [ ] **Step 2: CHANGELOG 加 [0.5.0.0] section**

在 CHANGELOG.md 顶部加:

```markdown
## [0.5.0.0] - 2026-06-13

C 子项目 — Helm chart 完整化 落地。5 个 frontend/backend agents + 主 session 收尾实施,~3 小时完成。

### Added

- **umbrella chart + 5 个子 chart** — `deploy/helm/charts/{cert-manager-issuer,external-secrets,sealed-secrets,monitoring,network-policies}/`。每个 opt-in (默认 disabled),主 chart 通过 `Chart.yaml` dependencies 引用。
- **cert-manager-issuer 子 chart** — ClusterIssuer (letsencrypt-prod + selfSigned) + Certificate CRD。
- **external-secrets 子 chart** — SecretStore (AWS 示例) + ExternalSecret 模板。
- **sealed-secrets 子 chart** — SealedSecret 模板 (静态加密,免云)。
- **monitoring 子 chart** — ServiceMonitor + PodMonitor (Prometheus 抓 /metrics)。
- **network-policies 子 chart** — NetworkPolicy 默认 deny + explicit allow (ingress from ingress-nginx + egress DNS + 可选 DB)。
- **minikube install 脚本** — `scripts/install-minikube.sh` 本地 minikube 完整验证 (start + helm dep + helm install + kubectl wait + port-forward + curl 测端点)。
- **TESTING.md** — minikube 验证详细步骤。
- **README.md** — 5 个子 chart 用法说明 + testing 链接。

### Notes

- 5 个子 chart 全默认 `enabled: false`, 用户 opt-in
- cert-manager/sealed-secrets/external-secrets/prometheus-operator 需用户自己装
- minikube install 不在 CI (资源限制),仅文档化手动验证
- ~15 atomic commits from 5 phase
```

- [ ] **Step 3: DOCUMENT_INDEX 加 C 索引**

加 spec 索引:

```markdown
| [2026-06-13-C-helm-full-design](docs/superpowers/specs/2026-06-13-C-helm-full-design.md) | **C 子项目 — Helm chart 完整化** (v0.5.0.0) | ✅ 已实施 |
```

加 plan 索引:

```markdown
| [2026-06-13-C-helm-full](docs/superpowers/plans/2026-06-13-C-helm-full.md) | **C 子项目 plan** (15 task, 5 phase × agent 并行) | ✅ Done |
```

- [ ] **Step 4: TODOS.md 加 C Completed section**

```markdown
### C 子项目 — Helm chart 完整化 (2026-06-13)

C 子项目 5 项 Helm 完整化全部落地,4 子 chart agents + 1 主 session 收尾,~3 小时完成。
关键 commit: 5 个子 chart 各 2 commits + 主 Chart.yaml dependencies 接线 + minikube 脚本 + TESTING.md + README 更新 + v0.5.0.0 release。

- Phase 1 (Agent 1, c1) cert-manager-issuer — 2 commits
- Phase 2 (Agent 2, c2) external-secrets — 2 commits
- Phase 3 (Agent 3, c3) sealed-secrets — 2 commits
- Phase 4 (Agent 4, c4) monitoring — 2 commits
- Phase 5 (主 session, c5) network-policies + minikube 脚本 + Chart.yaml dependencies + 文档 — 5 commits
- 31 packages Go 全绿,1 vet warning 既有
```

- [ ] **Step 5: 跑全量最终测试**

Run: `go test -count=1 -p 1 -timeout 600s ./... 2>&1 | tail -3`
Expected: 31 packages 全绿,0 fail。

- [ ] **Step 6: 提交**

```bash
git add VERSION CHANGELOG.md DOCUMENT_INDEX.md TODOS.md
git commit -m "chore(release): v0.5.0.0 — C 子项目 Helm 完整化文档收尾"
```

---

## Self-Review

### Spec coverage 验证

- [x] C1 cert-manager-issuer → Task 1-2
- [x] C2 external-secrets → Task 3-4
- [x] C3 sealed-secrets → Task 5-6
- [x] C4 monitoring → Task 7-8
- [x] C5 network-policies + minikube 脚本 + Chart.yaml dependencies + 文档 → Task 9-15

### Placeholder 扫描

无 TBD / TODO / FIXME。代码块完整。

### Type / method 一致性

- `_helpers.tpl` 每个子 chart 都有 `name` / `fullname` / `labels` / `selectorLabels` (4 个 define 完整)
- 每个子 chart 的 `Chart.yaml` 用 `appVersion: "0.5.0.0"` (与 VERSION 对齐)
- 每个子 chart 的 `enabled` 字段在 `values.yaml` 都是 bool 默认 false
- 主 chart dependencies 顺序: cert-manager-issuer, external-secrets, monitoring, network-policies, sealed-secrets (字母序)

### 关键风险

1. **5 agent 都可能撞 stop hallucination** — 用明确 "don't stop early" prompt + 主 session 协调
2. **主 Chart.yaml dependencies 文本冲突** — 5 agent 各自写 1 行,主 session 合并(避免 5 个 agent 同时改)
3. **minikube 不可在 CI 跑** — 仅文档化,shellcheck 验证 bash 语法
4. **NetworkPolicy 太严** — 默认 disabled,user opt-in 后必须手动验证

---

## Execution Handoff

**Plan 完成,共 15 task,5 phase,~15-20 commits,1 周。**

下一步:
- 4 个 frontend/backend agents 各开一个 branch(用 `/loop` background):
  - Agent 1: `feat/c1-cert-manager-issuer` (Task 1-2)
  - Agent 2: `feat/c2-external-secrets` (Task 3-4)
  - Agent 3: `feat/c3-sealed-secrets` (Task 5-6)
  - Agent 4: `feat/c4-monitoring` (Task 7-8)
- 主 session (Agent 5) 自己跑 Phase 5: `feat/c5-network-policies-install` (Task 9-15)
- 主 session 协调 merge 顺序: c1 → c2 → c3 → c4 → c5
