# DevOps Toolkit Helm Chart

最小骨架,不含 cert-manager / sealed-secrets 完整集成(留 C 阶段)。

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
