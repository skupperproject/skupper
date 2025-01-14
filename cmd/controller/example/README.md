# Installing the skupper controller

```
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_access_grant_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_access_token_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_attached_connector_anchor_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_attached_connector_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_certificate_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_connector_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_link_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_listener_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_router_access_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_secured_access_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/config/crd/bases/skupper_site_crd.yaml
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
kubectl apply -n west -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/site1.yaml
```

```
kubectl apply -n east -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/site2.yaml
```

# Expose backend in east site

```
kubectl apply -n east -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/connector.yaml
```

# Consume backend in west site

```
kubectl apply -n west -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/listener.yaml
```

# Link sites

Create a Grant in west site:

```
kubectl apply -n west -f https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/controller/example/access_grant.yaml
```

Wait until url and ca fields in status are set:

```
kubectl wait --for=condition=ready accessgrant/my-grant -n west && kubectl get accessgrant my-grant -n west -o yaml
```

Copy ca, code and url fields from grant status into the spec section of an accesstoken (see access_token.yaml), and apply that in site east

# Test connectivity

```
kubectl -n west port-forward deployment/frontend 8080:8080
```

Visit localhost:8080
