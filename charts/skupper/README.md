# Skupper

Skupper is a layer 7 service interconnect. It enables secure communications
across Kubernetes clusters and/or local systems with no VPNs or special firewall rules.

This chart installs the [Skupper](https://skupper.io) version 2 controller for
[Kubernetes](https://kubernetes.io) using the [Helm](https://helm.sh) package
manager.


## Prerequisites

- Kubernetes 1.25+
- Helm 3
- Skupper CRDs

### Installing CRDs

Install Skupper CRDs before deploying this chart. Choose one of the following methods:

**Using the skupper-crds Helm chart:**
```
helm install skupper-crds oci://quay.io/skupper/helm/skupper-crds
```

**Using kubectl:**
```
kubectl apply -f https://github.com/skupperproject/skupper/releases/latest/download/skupper-crds.yaml
```

## Using the chart

Deploy a cluster-scoped Skupper controller in the current namespace:
```
helm install skupper oci://quay.io/skupper/helm/skupper \
    --namespace skupper \
    --create-namespace
```

If you want to deploy the controller in a specific namespace:
```
helm install skupper oci://quay.io/skupper/helm/skupper \
    --namespace <custom-ns> \
    --create-namespace
```

Deploy a controller with namespace-scope in the current namespace using `--set rbac.clusterScoped=false`:
```
helm install skupper oci://quay.io/skupper/helm/skupper \
    --set rbac.clusterScoped=false
```

### Configuration

See [values.yaml](values.yaml) for the full list of configurable options.

Common customizations include image repositories, pull policies, pod
tolerations, and resource limits.

## Alternative Installation Methods

In addition to this Helm Chart, Skupper releases static manifest [YAML](../../cmd/controller/README.md) for
deploying both cluster and namespace-scoped controllers.

## Development

Install the chart from the local source:
```
helm install skupper ./charts/skupper
```

Or with namespace-scope:
```
helm install skupper ./charts/skupper --set rbac.clusterScoped=false
```
