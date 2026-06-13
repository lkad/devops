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