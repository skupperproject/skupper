# Skupper Site Controller

The site controller provides declarative methods to manage a Skupper VAN site using:

* Kubernetes ConfigMaps
* Tokens

## Managing a Skupper Site using ConfigMaps

ConfigMaps allow you to manage a Skupper site using a ConfigMap named `skupper-site` with the following parameters:

`data:name` -  A name for the site.

`data:cluster-local` -  (true/**false**) Set up skupper to only accept connections from within the local cluster.

`data:console` -  (**true**/false) Enable skupper console.

`data:console-authentication` -  ('openshift', 'internal', 'unsecured') Authentication method.

`data:console-user` -  Username for 'internal' option.

`data:console-password` - password for 'internal' option.

`data:console-ingress` - (route/loadbalancer/nodeport/nginx-ingress-v1/contour-http-proxy/ingress/none) Determines if/how console is exposed outside cluster. If not specified uses value of --ingress.

`data:create-network-policy` - (true/**false**) Create network policy to restrict access to skupper services exposed through this site to current pods in namespace

`data:edge` -  (true/**false**) Set up an edge skupper site.

`data:ingress` - (route/loadbalancer/nodeport/nginx-ingress-v1/contour-http-proxy/ingress/none) Setup Skupper ingress specific type. If not specified route is used when available, otherwise loadbalancer is used.

`data:ingress-annotations` - Annotations to add to skupper ingress separated by comma, e.g.: 'kubernetes.io/ingress.class=nginx'

`data:ingress-host` - Hostname by which the ingress proxy can be reached

`data:routers` - Number of router replicas to start

`data:router-console` - (true/false) Set up a Dispatch Router console (not recommended).

`data:router-debug-mode` - (true/**false**) Enable debug mode for router ('asan' or 'gdb' are valid values)

`data:router-logging` - Logging settings for router (e.g. trace,debug,info,notice,warning,error)

`data:router-mode` - (interior/edge) Skupper router-mode

`data:router-cpu` - CPU request for router pods

`data:router-memory` - Memory request for router pods

`data:router-cpu-limit` - CPU limit for router pods

`data:router-memory-limit` - Memory limit for router pods

`data:router-pod-affinity` - Pod affinity label matches to control placement of router pods

`data:router-pod-antiaffinity` - Pod antiaffinity label matches to control placement of router pods

`data:router-node-selector` - Node selector to control placement of router pods

`data:xp-router-max-frame-size` - Set  max frame size on inter-router listeners/connectors

`data:xp-router-max-session-frames` - Set  max session frames on inter-router listeners/connectors

`data:router-ingress-host` - Host through which node is accessible when using nodeport as ingress.

`data:router-service-annotations` - Annotations to add to skupper router service

`data:router-load-balancer-ip` - Load balancer ip that will be used for router service, if supported by cloud provider

`data:service-controller` - (true/false) Run the service controller.

`data:service-sync` - (**true**/false) Only relevant if the service controller is running. Determine if the service  controller participates in service synchronization.

`data:controller-cpu` - CPU request for controller pods

`data:controller-memory` - Memory request for controller pods

`data:controller-cpu-limit` - CPU limit for controller pods

`data:controller-memory-limit` - Memory limit for controller pods

`data:controller-pod-affinity` - Pod affinity label matches to control placement of controller pods

`data:controller-pod-antiaffinity` - Pod antiaffinity label matches to control placement of controller pods

`data:controller-node-selector` - Node selector to control placement of controller pods

`data:controller-ingress-host` - Host through which node is accessible when using nodeport as ingress.

`data:controller-service-annotations` - Annotations to add to skupper controller service

`data:controller-load-balancer-ip` - Load balancer ip that will be used for controller service, if supported by cloud provider

For example:

```yaml
apiVersion: v1
data:
  cluster-local: "false"
  console: "true"
  console-authentication: internal
  console-password: "barney"
  console-user: "rubble"
  edge: "false"
  name: skupp3r
  router-console: "true"
  service-controller: "true"
  service-sync: "true"
kind: ConfigMap
metadata:
  name: skupper-site
```

Note that `metadata:name` is name of the ConfigMap, and must be set to `skupper-site`.
