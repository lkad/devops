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
