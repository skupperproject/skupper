# Plan Review: Proxy Support for Non-Kube Sites

**Reviewed**: 2026-09-16
**Documents reviewed**: `.wayfinder/proxy-support-design-spec.md`, `.wayfinder/k8s-proxy-implementation.md`, `.wayfinder/map.md`

---

## Verdict

The plan is directionally sound: correct scope, correct surface area, correct mount patterns. Six tickets cover the real work. But it presents existing codebase scaffolding as new design decisions and points implementers at the wrong reference code. These gaps risk an implementer over-engineering a watch-based sync or writing redundant parsing code.

---

## Suggestions

### 1. Rewrite the reference implementation pointers

The plan's Notes section says to study `internal/kube/secrets/sync.go`. That file solves a dynamic-watch problem with checksums, ordinal tracking, and AMQP router sync. None of that applies to non-kube rendering, which runs once at boot and requires explicit reload for changes.

**Primary pattern to follow**: `internal/nonkube/common/fs_config_renderer.go`, specifically `createTlsCertificates()` (lines 335-474). It reads Secrets from `SiteState`, creates directories, writes files with appropriate permissions. The `createProxyProfiles()` function is a stripped-down version of this.

**Shared proxy wiring**: `internal/site/link.go`. This is already platform-agnostic. `Link.Apply` accepts a `ProxyConfig` struct and calls `qdr.ConfigureProxyProfile`. No K8s-specific code here.

**Demote** `internal/kube/secrets/sync.go` to "K8s-only dynamic machinery, not applicable to non-kube sites."

### 2. Add `ProxyProfilesPath` to `configurationDirectories`

In `internal/nonkube/common/fs_config_renderer.go`, the `configurationDirectories` slice (line 22) controls which directories are pre-created during rendering. `runtime/certs` is in the list. `runtime/proxies` is not.

The plan's `createProxyProfiles()` does its own `os.MkdirAll`, which works, but skipping the setup phase means the directory only exists when a proxy is configured. Add `api.ProxyProfilesPath` to `configurationDirectories` for consistency with how `runtime/certs` is handled.

### 3. Drop `ProxyProfileBasePath` initialization

Section 3.2.1 of the spec proposes initializing `ProxyProfileBasePath` separately from `SslProfileBasePath`. But the plan's own `getProxyConfig()` receives `sslProfileBasePath` as its parameter, and the two values are always identical.

The `ProxyProfileBasePath` field already exists on `FileSystemConfigurationRenderer` (line 60) but is never used. Adding initialization code for a dead field increases confusion. Use `SslProfileBasePath` throughout. If a separate proxy base path is needed later, add it then with an actual consumer.

### 4. Ticket 002: note that Secret loading already works

`SiteState.Secrets` is already populated by the existing `LoadIntoSiteState` pipeline, which reads `Secret-*.yaml` files from `input/resources/`. No new YAML-parsing code is needed.

The real design decision in ticket 002 is "use `type: kubernetes.io/basic-auth` as the discriminator to identify proxy Secrets," not "design a new schema."

### 5. Ticket 003: note that path constants already exist

`pkg/nonkube/api/environment.go` already defines:

```go
InputProxyProfilePath InternalPath = "input/proxies"   // line 17
ProxyProfilesPath     InternalPath = "runtime/proxies"  // line 21
```

These predate the plan. No code reads or writes to these paths yet, but the convention is already established. Ticket 003 ratifies it; it does not invent it. An implementer should know this scaffolding exists and is intentional.

### 6. Document the K8s error-handling divergence prominently

K8s `getProxyConfig` (site.go:1312) returns a hard error when the referenced Secret is missing, which fails the Link. The plan's non-kube `getProxyConfig` logs a warning and returns `nil`, allowing the Link to proceed without a proxy.

This is a reasonable divergence since non-kube sites lack dynamic reconciliation. But an implementer who reads the K8s code will see the hard error and might "fix" the warning-and-continue behavior without realizing it was deliberate. Add a code comment explaining the rationale.

### 7. Password file permissions differ from K8s

K8s uses `0644` for `password.txt` (`internal/kube/secrets/sync.go:358`). The plan specifies `0600`. This is intentional and arguably more correct, but should be documented since the router process must be able to read the file. Verify the router runs as the same user who owns the file, or relax to `0640`.

---

## Codebase orientation tasks

Before writing code, an implementer should complete these read-and-verify tasks to build a working mental model of the system.

### Task 1: Trace how a Link becomes router config today (without proxy)

Read these files in order:

1. `pkg/nonkube/api/site_state.go` — `linkMap()` method (line 267). Note the TODO comment and the empty `&site.ProxyConfig{}` being passed. This is where proxy resolution will be inserted.
2. `internal/site/link.go` — `NewLink()` (line 25) and `Link.Apply()` (line 41). Understand how `proxyConfig` flows into `qdr.ConfigureProxyProfile`. This code is shared between K8s and non-kube.
3. `internal/qdr/qdr.go` — `ConfigureProxyProfile()` (line 220) and `ProxyProfile` struct (line 588). Understand the `file:` prefix convention for password paths.

**Verify**: trace a Link with no proxy through `ToRouterConfig()` (site_state.go:296) and confirm the ProxyProfile section of the generated JSON is empty.

### Task 2: Understand how Secrets flow through SiteState

1. `pkg/nonkube/api/site_state.go` — `SiteState` struct (line 30). Note `Secrets map[string]*corev1.Secret`.
2. Find the loader that populates `SiteState.Secrets` from `input/resources/Secret-*.yaml` files. Confirm it handles `type: kubernetes.io/basic-auth` Secrets without special-casing (they go through the generic Secret path).
3. `pkg/nonkube/api/site_state.go` — `MarshalSiteState()` (line 449). Note line 484 marshals Secrets back out.

**Verify**: create a test `Secret-test-proxy.yaml` with `type: kubernetes.io/basic-auth` and `stringData` fields, load it through the pipeline, and confirm `siteState.Secrets["test-proxy"]` contains the expected data.

### Task 3: Understand the certificate-writing pattern you'll mirror

1. `internal/nonkube/common/fs_config_renderer.go` — `Render()` method (line 72). Note the order: create directories, create router config, create TLS certs, create tokens.
2. Same file — `createTlsCertificates()` (line 335). Study how it iterates Secrets, creates directories with `os.MkdirAll`, writes files with `os.WriteFile`, and handles errors. Your `createProxyProfiles()` will be a simplified version of this.
3. Same file — `configurationDirectories` slice (line 22). Note that `api.CertificatesPath` is listed but `api.ProxyProfilesPath` is not.

**Verify**: render a site with a Link and inspect the `runtime/certs/` directory structure. Confirm you understand which files are written and with what permissions.

### Task 4: Understand container mount setup

1. `internal/nonkube/compat/site_state_renderer.go` — `prepareContainers()` (line 217). Note the `FileMounts` slice with `runtime/certs` mounted to `/etc/skupper-router/runtime/certs` with `Options: []string{"z"}`.
2. `internal/nonkube/bundle/site_state_renderer.go` — `prepareContainers()` (line 100). Same pattern but with template variables (`{{.NamespacesPath}}`, `{{.Namespace}}`).
3. `pkg/container/client.go` — `FileMount` struct. Understand `Source`, `Destination`, `Options` fields.

**Verify**: render a site with podman, run `podman inspect <container>` and confirm the `runtime/certs` mount appears. Your `runtime/proxies` mount will be added alongside it.

### Task 5: Understand the platform path differences

1. `internal/nonkube/linux/site_state_renderer.go` — line 97-99. Note `SslProfileBasePath` is set to `siteHome` (absolute host path like `/home/user/.local/share/skupper/namespaces/default`).
2. `internal/nonkube/compat/site_state_renderer.go` — line 141-143. Note `SslProfileBasePath` is left empty, defaults to `${SSL_PROFILE_BASE_PATH}`, which the container's env var resolves to `/etc/skupper-router`.
3. `internal/nonkube/common/fs_config_renderer.go` — line 53-54. Note `DefaultSslProfileBasePath` and `DefaultProxyProfileBasePath` are both `${SSL_PROFILE_BASE_PATH}`.

**Verify**: understand why systemd router config contains absolute host paths in ProxyProfile.Password while container router config contains container-relative paths. Both work because the `file:` prefix tells the router to read from its own filesystem view.

### Task 6: Compare K8s proxy flow (read-only, for context)

1. `internal/kube/site/site.go` — `getProxyConfig()` (line 1312) and `newLink()` (line 1328). Note the hard error on missing Secret.
2. `internal/kube/secrets/sync.go` — `writeProxyProfile()` (line 348). Note `0644` permissions and the `file:` path construction.
3. `internal/kube/secrets/sync.go` — `handleProxyProfile()` (line 123). Note the checksum-based change detection. This is dynamic-watch machinery you will NOT be building.

**Verify**: confirm you understand why the K8s flow is more complex (dynamic Secret watching, AMQP router sync, ordinal tracking) and why the non-kube flow can be simpler (render once, reload explicitly).
