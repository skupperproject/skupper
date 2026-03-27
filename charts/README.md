## Helm Charts in the Skupper Project

### Quick Start

```bash
# 1. Install CRDs (required)
  helm install skupper-crds oci://quay.io/skupper/helm/skupper-crds
OR
  kubectl apply -f https://github.com/skupperproject/skupper/releases/latest/download/skupper-crds.yaml

# 2. Install Skupper controller
helm install skupper oci://quay.io/skupper/helm/skupper
```

### Charts

| Chart | Description |
|-------|-------------|
| [skupper-crds](skupper-crds/README.md) | Skupper Custom Resource Definitions (install first) |
| [skupper](skupper/README.md) | Skupper controller |
| [network-observer](network-observer/README.md) | Network observer for monitoring Skupper networks |
