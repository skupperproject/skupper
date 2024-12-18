# Skupper Network Observer

The skupper network observer is an application that attaches to the skupper
network in order to expose skupper network telemetry. When installed alongside
a skupper site it will collect operational data from ALL sites in the network
and expose them via the API and metrics that back the [Skupper
Console](https://github.com/skupperproject/skupper-console) web application.

This chart bootstraps a skupper network observer deployment on a
[Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh)
package manager.

## Prerequisites

- A kubernetes cluster or namespace with the skupper controller installed
- A skupper Site

## Configuration

By default, deploys the network-observer with skupper-issued TLS certificates,
HTTP Basic authentication (username and password are `skupper`) and no ingress.

### Ingress

Supports chosen service types (LoadBalancer, NodePort), Kubernetes Ingresses,
and Openshift Routes. Defaults to a ClusterIP Service.

### TLS

TLS is mandatory for this deployment. It can be configured as user provided, provided
by openshift or by the skupper controller.

### Authorization

The network observer pod contains a reverse proxy that handles authorization
and TLS termination for the read only application that binds only to localhost.
When authorization strategy is "basic", nginx is configured as the proxy, and
can be configured with user-provided htpasswd file contents or a secret name.
When authorization strategy is "openshift" an oauth2 proxy is used instead, and
is configured to use the cluster identity provider for authorization. Openshift
auth only works with ingress type Route.

