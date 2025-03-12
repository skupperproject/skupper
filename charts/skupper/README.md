# Skupper

Skupper is a layer 7 service interconnect. It enables secure communications
across Kubernetes clusters with no VPNs or special firewall rules.

This chart installs the [Skupper](https://skupper.io) version 2 controller for
[Kubernetes](https://kubernetes.io) using the [Helm](https://helm.sh) package
manager.


## Prerequisites

- Kubernetes 1.25+
- Helm 3

## Usage

A cluster-scoped skupper controller can be deployed with
```
helm install skupper oci://quay.io/skupper/helm/skupper \
    --namespace skupper \
    --create-namespace
```

To install the controller at a namespace-scope use `--set scope=namespace`
```
helm install skupper oci://quay.io/skupper/helm/skupper \
    --set scope=namespace
```

### CRDs

By default, the chart will install the Skupper CRDs required for the controller
to properly function. To install CRDs independently from the Helm chart, use
the `--skip-crds` flag with `helm install`.

### Image Overrides

The chart exposes overrides for the three images required to run a skupper site.
* `controllerImage`
* `kubeAdaptorImage`
* `routerImage`

## Alternative Installation Methods

In addition to this Helm Chart, Skupper releases static manifest yamls for
deploying both cluster and namespace-scoped controllers.

```
SKUPPER_VERSION=2.0.0
# Deploys a cluster scoped controller to the 'skupper' namespace.
kubectl apply -f "https://github.com/skupperproject/skupper/releases/download/$SKUPPER_VERSION/skupper-cluster-scope.yaml"
# Deploys a namespace scoped controller to the current context namespace.
kubectl apply -f "https://github.com/skupperproject/skupper/releases/download/$SKUPPER_VERSION/skupper-namespace-scope.yaml"
```

## Development

This Helm chart is generated from the Makefile at root of the [skupper
repository.](https://github.com/skupperproject/skupper)
```asciidoc
make generate-skupper-helm-chart
```
