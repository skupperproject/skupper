# Network Console Deployment

Examples of an independently deployable "network-console" application. Formerly
the flow-collector process was deployed alongside a skupper site as part of
site initiation with the `--enable-flow-collector` configuration, and the skupper
console web application (embedded in the flow-collector) was enabled via the
`--enable-console` configuration.

Moving forward in skupper v2, the network console will be decoupled from the
control plane. Instead, operators will choose to install the
**network-console** application independently. The **network-console**
application will consist of the **network-console-collector** process, and
optionally a **network-console-prometheus** deployment.

External Dependencies and privileges:

* A secret from an existing skupper site that contains
  certs and a connect.json detailing how to connect to a router.
* One of the following options for prometheus:
    * An installed **network-console-prometheus** deployment, preconfigured to
      scrape the collector metrics and to serve api queries for the collector.
      Includes role with access to read kube api resources in the installed
      namespace.
    * An external monitoring solution configured to scrape the `/metrics`
      endpoint on the network-console-collector api, and an open prometheus
      http v1 api endpoint for the network-console-collector to query (e.g.
      prometheus, thanos.)

## OpenShift deployment:

Should be a batteries included experience. Depends on the OpenShift service-ca
Operator for provisioning certificates and on the OpenShift oauth proxy for
authentication.

1. Make sure skupper is running in the current context's namesapce. `skupper status`.
1. Run `kubectl apply -f ./openshift/prometheus.yaml -f ./openshift/deployment.yaml` to deploy the resources.
1. Access the console via browser using the `network-console` route and your openshift credentials.

## Native k8s deployment:

This native kubernetes deployment is more of an example than a completed
solution. Users may want to plug in their own certificate management scheme and
their preferred ingress and authentication. This example assumes a slef-issued
certificate from cert-manager, a service of type LoadBalancer for ingress to
the network console, and no authentication layer.

1. Make sure [cert-manager](https://cert-manager.io/) is installed on your cluster. `kubectl get crd certificates.cert-manager.io`
1. Make sure skupper is running in the current context's namesapce. `skupper status`
1. Run `kubectl apply -f ./native/prometheus.yaml -f ./native/deployment.yaml` to deploy the resources.
1. See the running console either in browser or via the API.

## Podman:

A podman-compose project that runs the console unsecured.

1. Make sure skupper deployed as a podman site under your user. `skupper status --platform podman`
1. Run `podman-compose up -d`
1. The console should start at http://localhost:8080
