# Installing the skupper controller

```
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/deploy_cluster_scope.yaml
```

# Deploy application in two namespaces (or contexts)

E.g.

```
kubectl create namespace west
kubectl create deployment frontend --image quay.io/skupper/hello-world-frontend -n west
```

```
kubectl create namespace east
kubectl create deployment backend --image quay.io/skupper/hello-world-backend --replicas 3 -n east
```

# Create sites

```
kubectl apply -f -n west https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/site1.yaml
```

```
kubectl apply -f -n east https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/site2.yaml
```

# Expose backend in east site

```
kubectl apply -f -n east https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/connector.yaml
```

# Consume backend in west site

```
kubectl apply -f -n west https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/listener.yaml
```

# Link sites

## Option 1: work directly with yaml

Create a Grant in east site:

```
kubectl apply -f -n east https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/grant.yaml
```

Wait until url and ca fields in status are set:

```
kubectl get grant my-grant -n east -o yaml
```

Copy ca, url and secret fields from grant status into the spec section of a claim, and apply that in site west

## Option 2: use CLI to generate token

```
skupper token create --token-type=cert -n west token.yaml
```

```
kubectl apply -n east -f token.yaml
```

# Test connectivity

```
kubectl -n west port-forward deployment/frontend 8080:8080
```

Visit localhost:8080
