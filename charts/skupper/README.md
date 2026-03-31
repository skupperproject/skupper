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

### Access Type Configuration

The chart supports configuring how the Skupper router is exposed externally via
the `accessType` field on `RouterAccess` and `Site` resources. The controller
supports the following access types: `local`, `loadbalancer`, `route`,
`nodeport`, `ingress-nginx`, `contour-http-proxy`, and `gateway`.

By default the controller enables `local`, `loadbalancer`, and `route`. Use
the values below to change this behaviour.

| Value | Default | Description |
|---|---|---|
| `clusterHost` | `""` | IP or hostname of any cluster node. **Required** when `nodeport` is enabled. |
| `enabledAccessTypes` | `""` | Comma-separated list of enabled access types. Defaults to `local,loadbalancer,route` when empty. |
| `defaultAccessType` | `""` | Default access type for sites that do not specify one. Auto-selected when empty. |

#### Using NodePort

NodePort exposes the router on a high port of every cluster node. Set
`clusterHost` to the IP or hostname that clients can use to reach a node, and
include `nodeport` in `enabledAccessTypes`:

```bash
helm install skupper oci://quay.io/skupper/helm/skupper \
  --set clusterHost=192.168.1.100 \
  --set-literal enabledAccessTypes="local,loadbalancer,route,nodeport" \
  --set defaultAccessType=nodeport
```

Or with an override `values.yaml` file:

```yaml
clusterHost: "192.168.1.100"
enabledAccessTypes: "local,loadbalancer,route,nodeport"
defaultAccessType: "nodeport"
```

> **Note:** `defaultAccessType` is not mandatory. When omitted, the controller
> auto-selects the default access type (`route` on OpenShift, `loadbalancer`
> otherwise).

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
