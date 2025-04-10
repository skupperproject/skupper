# Skupper controller

Skupper releases static manifest yamls for deploying both cluster and namespace-scoped controllers.

```
SKUPPER_VERSION=v2-dev-release
# Deploys a cluster scoped controller to the 'skupper' namespace.
kubectl apply -f "https://github.com/skupperproject/skupper/releases/download/$SKUPPER_VERSION/skupper-cluster-scope.yaml"
# Deploys a namespace scoped controller to the current context namespace.
kubectl apply -f "https://github.com/skupperproject/skupper/releases/download/$SKUPPER_VERSION/skupper-namespace-scope.yaml"
```

Alternatively, clone this repo and generate the YAML files using:

```
make generate-skupper-deployment-cluster-scoped
make generate-skupper-deployment-namespace-scoped
```

You can also install using [Helm charts](../../charts/README.md).