---
id: 002
title: Secret YAML Schema Design
type: grilling
status: closed
blocks: []
created: 2026-09-15
resolved: 2026-09-15
---

## Question

What should the Secret YAML file schema be for proxy configuration in non-kube environments? Design the structure, validation rules, and storage location.

**Areas to cover**:

1. File path: Where do proxy Secret YAMLs live? (e.g., `~/.local/share/skupper/namespaces/<ns>/input/secrets/`)
2. YAML structure: Fields (`host`, `port`, `username`, `password`), types, required vs optional
3. Type discriminator: How to mark this as a proxy Secret vs TLS Secret? (K8s uses `type: kubernetes.io/basic-auth`)
4. Validation: What makes a proxy Secret valid? Port number range? Host format? Empty password allowed?
5. Discovery: How does the renderer find proxy Secrets? Filename convention? Metadata field?
6. Example: Concrete YAML showing a valid proxy Secret

**Constraints**:
- Must contain the same data fields as K8s Secret (`host`, `port`, `username`, `password`)
- Should feel consistent with existing non-kube Secret handling (see how TLS Secrets are stored)

**Why**: This is the user-facing interface - getting the schema wrong means breaking changes later.

**How to apply**: This schema becomes the input validation in `fs_config_renderer.go` and the example in user docs.

---

## Resolution

**File location**: Proxy Secrets stored at `~/.local/share/skupper/namespaces/<ns>/input/resources/Secret-<name>.yaml` alongside other resources (Site, Link, Listener, etc.). Use the existing resource marshaling pattern in `site_state.go`.

**YAML format**: Native Kubernetes Secret format with `stringData` for user-friendliness:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-proxy-config
  namespace: default
type: kubernetes.io/basic-auth
stringData:
  host: proxy.example.com
  port: "3128"
  username: myuser
  password: mypass
```

**Type discriminator**: Must use `type: kubernetes.io/basic-auth` to match K8s detection logic (see `internal/kube/secrets/manager.go:122`).

**Field requirements**:
- `host` (string) - **required** - proxy hostname or IP
- `port` (string) - **required** - must be parseable as a number, stored as string
- `username` (string) - **optional** - if present, password must also be present
- `password` (string) - **optional** - if present, username must also be present

**Unauthenticated proxies**: Omit `username` and `password` fields entirely:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: unauthenticated-proxy
type: kubernetes.io/basic-auth
stringData:
  host: proxy.internal.corp
  port: "8080"
```

**Validation rules**:
1. `type` must equal `kubernetes.io/basic-auth`
2. `host` and `port` must be present in `stringData` or `data`
3. `port` must be parseable as an integer (validate with `strconv.Atoi`)
4. If `username` present, `password` required (and vice versa)

**Discovery**: Secrets are discovered via the existing resource loading mechanism - all `Secret-*.yaml` files in `input/resources/` are loaded into `SiteState.Secrets` map (see `MarshalSiteState` in `site_state.go:484`).
