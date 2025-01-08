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

- A Skupper Version 2 Site running in the same Kuberentes Namesapce the network
observer is to be installed in.
- The Skupper Controller running and managing the Site.

## Configuration

By default, deploys the network-observer with skupper-issued TLS certificates,
HTTP Basic authentication (username and password are `skupper`) and no ingress.

### Ingress

By default the network-observer does not include an ingress. As a convenience,
the chart contains options that can help expose the service externally.

* Configure an ingress by setting `ingress.enabled=true` and setting appropriate
values under `ingress`.

Example values.yaml using the nginx ingress nginx controller with a
user-provided TLS certificate
```
ingress:
  enabled: true
  className: "nginx"
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "https"
  hosts:
    - host:  skupper-net-01.mycluster.local
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: skupper-net-01-tls
      hosts:
        - skupper-net-01.mycluster.local
```

* Configure an openshift route by setting `route.enabled=true`.

* Expose the service as type LoadBalancer `service.type=LoadBalancer`.

### TLS

TLS is mandatory for this deployment. It can be configured as user provided, provided
by openshift or by the skupper controller.

To use an existing TLS secret, overwrite `tls.secretName`.

To use an openshift generated service certificate, set
`tls.openshiftIssued=true` and `tls.skupperIssued=false`. An annotation will be
added to the service that should prompt openshift to provision a TLS secret.

### Authorization

The network observer pod contains a reverse proxy that handles authorization
and TLS termination for the read only application that binds only to localhost.
When authorization strategy is "basic", nginx is configured as the proxy, and
can be configured with user-provided htpasswd file contents or a secret name.
When authorization strategy is "openshift" an oauth2 proxy is used instead, and
is configured to use the cluster identity provider for authorization. Openshift
auth only works with ingress type Route.

To set a secure basic auth credentials run:
```
# Use htpasswd to generate a new password file
htpasswd -B -c passwords \
    my-username;

# Add a new secret with that password file
kubectl create secret generic my-custom-auth \
    --from-file=htpasswd=passwords;

# Point the chart at the new secret
helm install ... \
    --set auth.basic.create=false \
    --set auth.basic.secretName=my-custom-auth
```
