# Skupper controller

Skupper releases static manifest yamls for deploying both cluster and namespace-scoped controllers.

```
SKUPPER_VERSION=2.0.0
# Deploys a cluster scoped controller to the 'skupper' namespace.
kubectl apply -f "https://github.com/skupperproject/skupper/releases/download/$SKUPPER_VERSION/skupper-cluster-scope.yaml"
# Deploys a namespace scoped controller to the current context namespace.
kubectl apply -f "https://github.com/skupperproject/skupper/releases/download/$SKUPPER_VERSION/skupper-namespace-scope.yaml"
```

You can also install 