### Description

This Helm chart is designed to simplify the deployment of Skupper in a Kubernetes environment. 
It ensures that the Skupper Custom Resource Definitions (CRDs) and the Skupper controller 
are correctly installed and configured.

### Usage:

### Scope: namespace
To deploy Skupper using this Helm chart, simply run the following command, specifying your 
namespace:

```
helm install skupper . --set scope=namespace --namespace <your-namespace>
```

If the namespace is not specified it will be deployed in the current namespace.

### Scope: cluster
To deploy Skupper using this Helm chart, simply run the following command:

```
helm install skupper . --set scope=cluster 
```

Skupper will be installed by default in a namespace called `skupper`, if you want to specify a different name, 
just set the value `controllerNamespace` and the chart will create that namespace deploying skupper there.

```
helm install skupper . --set scope=cluster controllerNamespace=my-namespace  
```

### How to uninstall the helm chart
```
helm uninstall skupper
``` 

The CRDs have to be removed manually, given that Helm does not delete them by design.