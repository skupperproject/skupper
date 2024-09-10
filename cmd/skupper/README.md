# Installing the skupper controller

```
kubectl create namespace skupper
kubectl config set-context --current --namespace skupper
```

```
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_access_grant_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_access_token_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_attached_connector_anchor_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_attached_connector_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_certificate_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_connector_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_link_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_listener_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_router_access_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_secured_access_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/skupperproject/skupper/v2/api/types/crds/skupper_site_crd.yaml
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
skupper site create site1 --enable-link-access -n west
```

```
skupper site create site2  -n east
```

# Expose backend in east site (create a connector)

```
skupper connector create backend 8080 --routing-key backend --selector app=backend -n east
```

# Consume backend in west site (create a listener)

```
skupper listener create backend 8080 --host backend --routing-key backend -n west
```

# Link sites

Create a AccessGrant in west site and generate a file with the accessToken to redeem in east site:

```
skupper token issue token ~/token.yaml -n west
```

```
skupper token redeem ~/token.yaml -n east
```

### Alternative: generate a link custom resource

Generate a file with a link Custom resource and its certificate in west site
```
skupper link generate -n west > ~/linktowest.yaml
```
Apply the file in east site: 

```
kubectl apply -n east -f ~/linktowest.yaml
```


# Test connectivity

```
kubectl -n west port-forward deployment/frontend 8080:8080
```

Visit localhost:8080
