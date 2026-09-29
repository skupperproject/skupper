# Task 5 Report: Platform path differences for SSL/proxy profile base paths

## Data flow summary

```
Linux (systemd):
  SiteStateRenderer.Render()                    # linux/site_state_renderer.go:96
    └─ siteHome = GetHostSiteHome(site)         # e.g. /home/user/.local/share/skupper/namespaces/default
         └─ FileSystemConfigurationRenderer{SslProfileBasePath: siteHome}
              └─ ToRouterConfig(siteHome, "linux")
                   └─ SslProfile.CaCertFile = "/home/user/.local/share/skupper/namespaces/default/runtime/certs/<name>/ca.crt"
                   └─ ProxyProfile.Password  = "file:/home/user/.local/share/skupper/namespaces/default/runtime/proxies/<name>/password.txt"

Compat (podman/docker):
  SiteStateRenderer.Render()                    # compat/site_state_renderer.go:141
    └─ FileSystemConfigurationRenderer{SslProfileBasePath: ""}  # empty → defaults
         └─ Render() applies default: "${SSL_PROFILE_BASE_PATH}"
              └─ ToRouterConfig("${SSL_PROFILE_BASE_PATH}", "podman")
                   └─ SslProfile.CaCertFile = "${SSL_PROFILE_BASE_PATH}/runtime/certs/<name>/ca.crt"
                   └─ ProxyProfile.Password  = "file:${SSL_PROFILE_BASE_PATH}/runtime/proxies/<name>/password.txt"

Container env var resolves at runtime:
  SSL_PROFILE_BASE_PATH=/etc/skupper-router     # compat/site_state_renderer.go:228
  + FileMount: host:runtime/certs → container:/etc/skupper-router/runtime/certs
```

## Step-by-step trace

### 1. Linux (systemd) — `internal/nonkube/linux/site_state_renderer.go:96-100`

```go
siteHome := api.GetHostSiteHome(s.siteState.Site)
s.configRenderer = &common.FileSystemConfigurationRenderer{
    SslProfileBasePath: siteHome,
    Platform:           string(types.PlatformLinux),
}
```

`GetHostSiteHome()` (`pkg/nonkube/api/environment.go:80`) resolves to an absolute host path:
- Non-root: `~/.local/share/skupper/namespaces/<namespace>` (via `XDG_DATA_HOME`)
- Root: `/var/lib/skupper/namespaces/<namespace>`

The `SslProfileBasePath` is set to this absolute path. The router process on systemd runs directly on the host, so it can read files at these absolute paths.

### 2. Compat (podman/docker) — `internal/nonkube/compat/site_state_renderer.go:141-143`

```go
s.configRenderer = &common.FileSystemConfigurationRenderer{
    Platform: string(platform),
}
```

`SslProfileBasePath` is **not set** — left as zero-value `""`.

### 3. Default resolution — `internal/nonkube/common/fs_config_renderer.go:76-78`

In `Render()`:

```go
if c.SslProfileBasePath == "" {
    c.SslProfileBasePath = DefaultSslProfileBasePath  // "${SSL_PROFILE_BASE_PATH}"
}
```

The literal string `${SSL_PROFILE_BASE_PATH}` is embedded into the generated router config JSON. This is **not** a Go template or os.ExpandEnv call — it's a raw string that the skupper-router process resolves at runtime via its own environment.

Note: `ProxyProfileBasePath` has the same default (`${SSL_PROFILE_BASE_PATH}`) at line 54, but is **never used** — proxy path construction goes through `SslProfileBasePath` via `linkMap()`.

### 4. Config generation — `internal/nonkube/common/fs_config_renderer.go:318`

```go
c.RouterConfig = siteState.ToRouterConfig(c.SslProfileBasePath, c.Platform)
```

`ToRouterConfig()` passes `sslProfileBasePath` into `linkMap()` and `bindings()`, which build paths like:

```go
// link.go:70 — SSL profile
path.Join(sslProfilePath, name, "ca.crt")

// qdr.go:228 — Proxy profile password
path_.Join("file:", path, name, "password.txt")
```

### 5. Container env var — `internal/nonkube/compat/site_state_renderer.go:228`

```go
Env: map[string]string{
    "SSL_PROFILE_BASE_PATH": "/etc/skupper-router",
    ...
}
```

The container's environment sets `SSL_PROFILE_BASE_PATH=/etc/skupper-router`. The router process expands `${SSL_PROFILE_BASE_PATH}` in its config to this value.

### 6. Volume mounts bridge host to container — `compat/site_state_renderer.go:234-244`

```go
FileMounts: []container.FileMount{
    {
        Source:      path.Join(siteConfigPath, string(api.CertificatesPath)),   // host: .../runtime/certs
        Destination: "/etc/skupper-router/runtime/certs",                      // container
        Options:     []string{"z"},
    },
}
```

The host directory `<siteHome>/runtime/certs` is mounted into the container at `/etc/skupper-router/runtime/certs`. The generated config references `${SSL_PROFILE_BASE_PATH}/runtime/certs/<name>/ca.crt`, which resolves to `/etc/skupper-router/runtime/certs/<name>/ca.crt` inside the container — exactly where the mount places the files.

### 7. Bundle renderer — `internal/nonkube/bundle/site_state_renderer.go:100-127`

Same pattern as compat but with Go template variables (`{{.NamespacesPath}}`, `{{.Namespace}}`). The env var is identically set to `/etc/skupper-router` (line 110).

### 8. K8s comparison — `internal/kube/site/site.go:201-202`

K8s uses hardcoded constants:

```go
const SSL_PROFILE_PATH = "/etc/skupper-router-certs"
const PROXY_PROFILE_PATH = "/etc/skupper-router-proxies"
```

These are container-internal paths. K8s mounts Secrets directly into the pod at these paths via volume mounts managed by the operator.

## Verification

**Confirmed**: the path divergence is correct and intentional.

| Platform | SslProfileBasePath value | Example SslProfile.CaCertFile | Example ProxyProfile.Password |
|---|---|---|---|
| Linux/systemd | `/home/user/.local/share/skupper/namespaces/default` | `/home/user/.../runtime/certs/link-a-profile/ca.crt` | `file:/home/user/.../runtime/proxies/my-proxy/password.txt` |
| Podman/Docker | `${SSL_PROFILE_BASE_PATH}` (literal) | `${SSL_PROFILE_BASE_PATH}/runtime/certs/link-a-profile/ca.crt` | `file:${SSL_PROFILE_BASE_PATH}/runtime/proxies/my-proxy/password.txt` |
| K8s | `/etc/skupper-router-certs` (hardcoded) | `/etc/skupper-router-certs/link-a-profile/ca.crt` | `file:/etc/skupper-router-proxies/my-proxy/password.txt` |

**Why both work**: The `file:` prefix tells the skupper-router to read from its own filesystem view. On systemd, the router runs directly on the host, so absolute host paths are valid. In containers, volume mounts map host files into the container filesystem at paths that match what `${SSL_PROFILE_BASE_PATH}` expands to.

## Key observations for proxy implementation

1. **No separate ProxyProfileBasePath needed**: The `ProxyProfileBasePath` field exists on `FileSystemConfigurationRenderer` (line 60) but is never read. `SslProfileBasePath` is the single path that flows through `linkMap()` → `NewLink()` → `Link.Apply()` → `ConfigureProxyProfile()`. The plan-review (suggestion 3) correctly identifies this as dead code.

2. **Password path construction**: `ConfigureProxyProfile()` (`qdr.go:228`) builds `file:<basePath>/runtime/proxies/<name>/password.txt`. For this to work, the `proxyConfig.ProfilePath` must be set to the same `sslProfileBasePath` that's used for SSL profiles. The non-kube `getProxyConfig()` should receive and pass through whatever `sslProfileBasePath` value is in use.

3. **Volume mount addition**: For container platforms, a new `FileMount` must be added in both `compat/site_state_renderer.go:234` and `bundle/site_state_renderer.go:116`:
   ```go
   {
       Source:      path.Join(siteConfigPath, string(api.ProxyProfilesPath)),  // host: .../runtime/proxies
       Destination: "/etc/skupper-router/runtime/proxies",                     // container
       Options:     []string{"z"},
   }
   ```

4. **Linux needs no mount**: On systemd, absolute host paths work directly. The router reads files from the same filesystem where they were written. No additional configuration needed beyond writing `password.txt` to the correct path.

5. **Path constants already exist**: `pkg/nonkube/api/environment.go` defines both `InputProxyProfilePath = "input/proxies"` (line 17) and `ProxyProfilesPath = "runtime/proxies"` (line 21). These are ready to use.
