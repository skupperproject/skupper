# Skupper Site Controller

The site controller provides declarative methods to manage a Skupper VAN site using:

* Kubernetes ConfigMaps
* Tokens


## Managing a Skupper Site using ConfigMaps

ConfigMaps allow you to manage a Skupper site using the following parameters:

`metadata:name` -  the site controller name in OpenShift

`data:name` -  the VAN deployment or site name

`data:cluster-local` -  (true/false) Set up skupper to only accept connections from within the local cluster.

`data:console` -  (**true**/false) Enable skupper console

`data:console-authentication` -  ('openshift', 'internal', 'unsecured') Autentication method

`data:console-user` -  username for 'unsecured' option

`data:console-password` - password for 'unsecured' option

`data:edge` -  (true/false) Set up an edge skupper site

`data:router-console` - (true/false) Set up a Dispatch Router console (not recommended)

`data:service-controller` - 

`data:service-sync` -  


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
