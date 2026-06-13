# DevOps Toolkit Helm Chart

umbrella chart + 5 个子 chart (cert-manager-issuer / external-secrets / sealed-secrets / monitoring / network-policies),每个 opt-in。

## 用法

```bash
helm install devops ./deploy/helm \
  --set postgres.host=db.example.com \
  --set backup.s3Bucket=my-backup-bucket
```

## Secret 注入

Secret 通过 ExternalSecrets / SealedSecrets 注入,详见 `templates/secret.yaml` 的 annotations。
```yaml
# 示例: ExternalSecret 引用 AWS Secrets Manager
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: devops-toolkit
spec:
  secretStoreRef:
    name: aws-secrets-manager
    kind: ClusterSecretStore
  target:
    name: devops-toolkit-secrets
  data:
    - secretKey: APP_JWT_SECRET
      remoteRef:
        key: devops/prod
        property: jwt_secret
```

## 验证

```bash
helm template ./deploy/helm
helm lint ./deploy/helm
```

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
