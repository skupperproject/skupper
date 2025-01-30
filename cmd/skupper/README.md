
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
skupper connector create backend 8080 -n east
```

# Consume backend in west site (create a listener)

```
skupper listener create backend 8080 -n west
```

# Link sites

Create a AccessGrant in west site and generate a file with the AccessToken:

```
skupper token issue ~/my-token.yaml -n west
```

Copy token.yaml file to east site and redeem:

```
skupper token redeem ~/my-token.yaml -n east
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
