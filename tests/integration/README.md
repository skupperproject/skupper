# Integration tests

Go integration tests that run Skupper components against a real Kubernetes API server
using [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) (kube-apiserver +
etcd), without needing a full cluster.

These sit between unit tests (fake clients, synchronous event processing) and Ansible E2E
tests under `tests/e2e/` (real clusters, cross-site networking).

## Layout

Mirrors `internal/` so kube and nonkube integration tests can live alongside their
production packages.

| Directory | Tests |
|-----------|-------|
| `kube/controller/` | Skupper kube controller (`internal/kube/controller`, `cmd/controller`) |
| `nonkube/controller/` | (future) nonkube controller (`internal/nonkube/controller`) |

## Prerequisites

The `setup-envtest` version is pinned in the root `go.mod` `tool` directive (matching
controller-runtime release-0.21 / k8s 1.33). Run from the repository root:

```bash
go tool setup-envtest use -i 1.33.0
```

Or let `make test-integration` download binaries on first run.

To pre-download Kubernetes test binaries without running tests:

```bash
go tool setup-envtest use -i 1.33.0
```


## Run

From the repository root:

```bash
make -C tests test-integration
```

Or from `tests/`:

```bash
make test-integration
```

Or directly:

```bash
export KUBEBUILDER_ASSETS=$(go tool setup-envtest use 1.33.0 -p path)
go test -tags=integration -v ./tests/integration/kube/controller/...
```

Default `make test` does **not** run these (they use the `integration` build tag and take
~1 minute).

## Notes

- Tests start a shared controller instance and a fresh envtest apiserver per package run.
- Gateway, Contour, and OpenShift Route CRDs are not installed; related watcher errors in
  logs are expected and harmless for current scenarios.
- A teardown warning about kube-apiserver shutdown may appear after tests pass; this is a
  known envtest quirk and does not indicate test failure.
