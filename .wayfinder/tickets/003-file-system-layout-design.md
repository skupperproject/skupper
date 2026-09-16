---
id: 003
title: File System Layout Design
type: grilling
status: closed
blocks: []
created: 2026-09-15
resolved: 2026-09-15
---

## Question

What is the complete file system layout for proxy profile storage and rendering in non-kube environments? Design paths, permissions, and directory structure for all three platforms (podman, docker, systemd).

**Areas to cover**:

1. **Input paths**: Where proxy Secret YAML files are stored (user-writable)
2. **Rendered paths**: Where `password.txt` gets written during config rendering
3. **Runtime paths**: What the router sees inside containers vs systemd (e.g., `/etc/skupper-router-proxies/<name>/password.txt`)
4. **Permissions**: File modes for Secret YAML (input), password.txt (rendered), directories
5. **Path constants**: Should we define constants like K8s has `PROXY_PROFILE_PATH`?
6. **Container mounts**: For podman/docker, what directories get mounted? Does this change existing mounts or add new ones?
7. **Namespace isolation**: How do multiple namespaces keep proxy configs separate?

**Constraints**:
- Router must see identical paths regardless of platform (`/etc/skupper-router-proxies/<name>/password.txt`)
- Follow existing non-kube patterns (see how `/config/certs/` works)
- Respect user-writable vs system-rendered separation

**Why**: Wrong paths mean router can't find passwords, or secrets leak across namespaces.

**How to apply**: These paths become constants in `common/fs_config_renderer.go` and volume mount specs in container installers.

---

## Resolution

### Path Structure

**Follows cert pattern exactly** - use the already-defined constants `InputProxyProfilePath = "input/proxies"` and `ProxyProfilesPath = "runtime/proxies"` from `pkg/nonkube/api/environment.go:17,21`.

**Input (user-writable)**:
- Secret YAML: `~/.local/share/skupper/namespaces/<ns>/input/resources/Secret-<name>.yaml`
  - Per ticket #002 decision

**Rendered (system-written)**:
- Password files: `~/.local/share/skupper/namespaces/<ns>/runtime/proxies/<profile-name>/password.txt`
  - Directory: `runtime/proxies/<profile-name>/` (mode 0755)
  - File: `password.txt` (mode 0600 - owner read/write only)

### Platform-Specific Router Paths

**Podman/Docker (containers)**:
- Container mount: Host `runtime/proxies/` → Container `/etc/skupper-router/runtime/proxies/`
- Router config paths: `/etc/skupper-router/runtime/proxies/<name>/password.txt`
- ProxyConfig.ProfilePath: `/etc/skupper-router/runtime/proxies`
- Router sees: `file:/etc/skupper-router/runtime/proxies/<name>/password.txt`

**Linux/Systemd (native process)**:
- No container, router reads directly from filesystem
- Router config paths: `~/.local/share/skupper/namespaces/<ns>/runtime/proxies/<name>/password.txt`
- ProxyConfig.ProfilePath: `~/.local/share/skupper/namespaces/<ns>/runtime/proxies`
- Router sees: `file:<absolute-path>/runtime/proxies/<name>/password.txt`

**Pattern**: SslProfileBasePath (and ProxyProfileBasePath) is platform-specific:
- Containers: `/etc/skupper-router`
- Systemd: Actual site home directory (from `api.GetHostSiteHome()`)

See `linux/site_state_renderer.go:98` where systemd sets `SslProfileBasePath: siteHome`.

### File Permissions

- `runtime/proxies/` directory: **0755** (owner rwx, group/other rx)
- `runtime/proxies/<name>/` directory: **0755**
- `runtime/proxies/<name>/password.txt`: **0600** (owner rw only)
  - Stricter than cert files (0640) since passwords are authentication credentials

### Container Volume Mounts

**Add to container configuration** (in `compat/site_state_renderer.go:235-244` pattern):

```go
{
    Source:      path.Join(siteConfigPath, string(api.ProxyProfilesPath)),
    Destination: "/etc/skupper-router/runtime/proxies",
    Options:     []string{"z"},
}
```

This mirrors the existing cert mount at line 241-244.

### Path Constants

**Already defined** in `pkg/nonkube/api/environment.go`:
- Line 17: `InputProxyProfilePath InternalPath = "input/proxies"`
- Line 21: `ProxyProfilesPath InternalPath = "runtime/proxies"`

**New constant needed** in `common/fs_config_renderer.go`:
```go
const DefaultProxyProfileBasePath = "${PROXY_PROFILE_BASE_PATH}"
```

Mirrors `DefaultSslProfileBasePath` pattern (line 53).

### Namespace Isolation

Automatic via existing namespace directory structure:
- Each namespace has its own `runtime/proxies/` directory
- Paths are: `~/.local/share/skupper/namespaces/<namespace>/runtime/proxies/...`
- No cross-namespace access possible

### Summary Table

| Aspect | Podman/Docker | Linux/Systemd |
|--------|---------------|---------------|
| Host password path | `~/.local/.../ns/runtime/proxies/<name>/password.txt` | `~/.local/.../ns/runtime/proxies/<name>/password.txt` |
| Router sees | `/etc/skupper-router/runtime/proxies/<name>/password.txt` | `~/.local/.../ns/runtime/proxies/<name>/password.txt` |
| ProfilePath value | `/etc/skupper-router/runtime/proxies` | `~/.local/.../ns/runtime/proxies` |
| Mount required | Yes (add to container config) | No |
| Config paths are | Relative to mount point | Absolute filesystem paths |
