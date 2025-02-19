# Simple Declarative Example

This example demonstrates how to connect two sites declaratively, without relying on the CLI.

## Installing the skupper controller

The controller can be installed using the Helm chart, with detailed instructions provided
 [here](https://github.com/skupperproject/skupper/blob/main/charts/README.md)  

## Deploy application in two namespaces (or contexts)

E.g.

```
kubectl create namespace west
kubectl create deployment frontend --image quay.io/skupper/hello-world-frontend -n west
```

```
kubectl create namespace east
kubectl create deployment backend --image quay.io/skupper/hello-world-backend --replicas 3 -n east
```

## Create sites

```
kubectl apply -n west -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/site1.yaml
```

```
kubectl apply -n east -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/site2.yaml
```

## Expose backend in east site

```
kubectl apply -n east -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/connector.yaml
```

## Consume backend in west site

```
kubectl apply -n west -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/listener.yaml
```

## Link sites

Create a Grant in west site:

```
kubectl apply -n west -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/access_grant.yaml
```

Wait until url and ca fields in status are set:

```
kubectl wait --for=condition=ready accessgrant/my-grant -n west && kubectl get accessgrant my-grant -n west -o yaml
```

Copy ca, code and url fields from grant status into the spec section of an accesstoken (see access_token.yaml), and apply that in site east

## Test connectivity

```
kubectl -n west port-forward deployment/frontend 8080:8080
```

Visit localhost:8080


----
**Additional Notes**
- An equivalent example using the CLI, is provided in the [CLI Example](https://github.com/skupperproject/skupper/blob/main/cmd/skupper/README.md)