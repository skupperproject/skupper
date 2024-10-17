### Description

This Helm chart is designed to simplify the deployment of Skupper in a Kubernetes environment. 
It ensures that the Skupper Custom Resource Definitions (CRDs) and the Skupper controller 
are correctly installed and configured.

### Usage:

### Scope: namespace
To deploy Skupper using this Helm chart, simply run the following command, specifying your 
namespace:

```
helm install skupper-namespace-setup . --set scope=namespace --namespace <your-namespace>
```

If the namespace is not specified it will be deployed in the current namespace.

### Scope: cluster
To deploy Skupper using this Helm chart, simply run the following command:

```
helm install skupper-namespace-setup . --set scope=cluster 
```

### How to uninstall the helm chart
```
helm uninstall skupper-namespace-setup
``` 