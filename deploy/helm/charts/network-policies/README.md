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
kubectl get networkpolicy -l app.kubernetes.io/name=devops-toolkit
```
