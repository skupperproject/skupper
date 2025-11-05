# Code Generation

Skupper uses code generation tools to generate various boilerplate across the
codebase.

## Kubernetes API Types

Skupper defines go types for its kubernetes API in packages under under
`github.com/skupperproject/skupper/pkg/apis/...`. These types are annotated
with k8s code generation specific comments. These types and comments are used
as the basis for generating all of the code needed to work with these types
natively with the `k8s.io/client-go` client.

* Kubernetes client

The `scripts/update-codegen.sh` script depends on scripts bundled with the
`k8s.io/code-generator` go module. It generates deepcopy helper functions
alongside the api types under pkg/apis/ as well as clients, informers and
listers  under `github.com/skupperproject/skupperproject/pkg/generated/client`.

In an abundance of caution to avoid contamination from the dev host, we suggest
running in a container. Example:

```
GO_VERSION=1.24.9

podman run -v $(pwd):/work:rw,Z \
  -w /work \
  "docker.io/golang:$GO_VERSION" \
  bash -c 'go mod download && ./scripts/update-codegen.sh'
```


* Kubernetes CRD Spec

The CRD Specifications are presently maintained manually, and must be kept in
sync with the go types. Generating the CRD specifications from the go types can
be a useful tool for comparison.

```
go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0
controller-gen crd \
    paths=./pkg/apis/skupper/v2alpha1/... \
    output:crd:dir=./generated

# diff generated/skupper.io_sites.yaml ./config/crd/bases/skupper_site_crd.yaml
```

## Network Observer API Types

The network observer API is specified at
`cmd/network-observer/spec/openapi.yaml`. This specification is used to
generate the API types and routing the network-observer uses. After updating
the specification or upgrading the oapi-codegen library, run the code
generation with `go generate ./cmd/network-observer`.
