# Skupper Site Controller

The site controller provides declarative methods to manage a Skupper VAN site using:

* Kubernetes ConfigMaps
* Tokens


## Managing a Skupper Site using ConfigMaps

ConfigMaps allow you to manage a Skupper site using a ConfigMap named `skupper-site` with the following parameters:

`data:name` -  A name for the site.

`data:cluster-local` -  (true/false) Set up skupper to only accept connections from within the local cluster.

`data:console` -  (**true**/false) Enable skupper console.

`data:console-authentication` -  ('openshift', 'internal', 'unsecured') Autentication method.

`data:console-user` -  Username for 'internal' option.

`data:console-password` - password for 'internal' option.

`data:edge` -  (true/false) Set up an edge skupper site.

`data:router-console` - (true/false) Set up a Dispatch Router console (not recommended).

`data:service-controller` - (true/false) Run the service controller.

`data:service-sync` - (**true**/false) Only relevant if the service controller is running. Determine if the service  controller participates in service synchronization.


For example:

```
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
