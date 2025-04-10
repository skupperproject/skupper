# Skupper

Skupper is a layer 7 service interconnect. It enables secure communications
across Kubernetes clusters and/or local systems with no VPNs or special firewall rules.

This chart installs the [Skupper](https://skupper.io) version 2 controller for
[Kubernetes](https://kubernetes.io) using the [Helm](https://helm.sh) package
manager.


## Prerequisites

- Kubernetes 1.25+
- Helm 3

## Using the chart

Deploy a cluster-scoped Skupper controller in the current namespace:
```
helm install skupper oci://quay.io/skupper/helm/skupper
```

If you want to deploy the controller in a specific namespace:
```
helm install skupper oci://quay.io/skupper/helm/skupper \
    --namespace <custom-ns> \
    --create-namespace
```

Deploy a controller with namespace-scope in the current namespace using  `--set scope=namespace`:
```
helm install skupper oci://quay.io/skupper/helm/skupper \
    --set scope=namespace
```

### CRDs

By default, the chart installs the Skupper CRDs required by the controller
to properly function.  If you want to install CRDs separately from the Helm chart, use
the `--skip-crds` flag with `helm install`.

### Image Overrides

The chart exposes overrides for the three images required to run a skupper site.

Example values.yaml file:
```
controllerImage:    examplemirror.acme.com/skupper/controller:2.0.0
kubeAdaptorImage:   examplemirror.acme.com/skupper/kube-adaptor:2.0.0
routerImage:        examplemirror.acme.com/skupper/skupper-router:3.3.0
```

## Alternative Installation Methods

In addition to this Helm Chart, Skupper releases static manifest [YAML](../../cmd/controller/README.md) for
deploying both cluster and namespace-scoped controllers.

## Development

The skupper chart is generated from common config files, so you will need to run:
```asciidoc
make generate-skupper-helm-chart
```

This action will create a `skupper` chart inside the `charts` directory, that 
you can install with a clustered scope with:
```
helm install skupper ./skupper --set scope=cluster
```
Other option is to install it in a namespaced scope:
```
helm install skupper ./skupper --set scope=namespace
```

Check the `values.yaml` to modify the image tag of the controller, kube-adaptor and router images. 
