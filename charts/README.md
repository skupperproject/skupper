## Helm Charts in the Skupper Project

### Skupper chart

The skupper chart is generated from common config files, so you will need to run:
```asciidoc
make generate-skupper-helm-chart
```

This action will create a `skupper` chart inside the `charts` directory, that 
you can install with a clustered scope with:
```
helm install skupper ./skupper --set scope=cluster
```
Other option is to install it in a namespaced scope:
```
helm install skupper ./skupper --set scope=namespace
```

Check the `values.yaml` to modify the image tag of the controller, kube-adaptor and router images. 


### Network-observer chart

[Instructions on how to deploy the network-observer chart.](https://github.com/skupperproject/skupper/blob/main/charts/network-observer/README.md) 

