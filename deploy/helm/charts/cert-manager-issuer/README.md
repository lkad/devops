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
