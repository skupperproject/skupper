# HTTP Proxy Support for Non-Kube Sites - Design Specification

**Version**: 1.0  
**Date**: 2026-09-15  
**Status**: Ready for Implementation

## Executive Summary

This specification describes how to extend HTTP proxy support for inter-site Links from Kubernetes environments to local system sites running on podman, docker, or linux/systemd platforms. The design mirrors the existing K8s implementation pattern, maintaining API consistency while adapting to file-based configuration storage.

**Key Principle**: The Link YAML `settings: {proxy-configuration: <name>}` reference pattern remains identical to K8s. Only the backing storage changes from K8s Secret API to file-based Secret YAML.

---

## 1. User-Facing API

### 1.1 Proxy Secret YAML Format

**Location**: `~/.local/share/skupper/namespaces/<namespace>/input/resources/Secret-<name>.yaml`

**Schema** (identical to K8s):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-proxy-config
  namespace: default
type: kubernetes.io/basic-auth
stringData:
  host: proxy.example.com      # Required
  port: "3128"                  # Required (string)
  username: myuser              # Optional
  password: <your-password>    # Optional (must pair with username)
```

**Type Discriminator**: `type: kubernetes.io/basic-auth` (same as K8s)

**Field Requirements**:
- `host` (string) - **Required** - proxy hostname or IP
- `port` (string) - **Required** - must be parseable as integer, stored as string
- `username` (string) - **Optional** - if present, `password` must also be present
- `password` (string) - **Optional** - if present, `username` must also be present

**Unauthenticated Proxies**: Omit `username` and `password` fields entirely.

**Validation Rules**:
1. `type` must equal `kubernetes.io/basic-auth`
2. `host` and `port` must be present
3. `port` must be parseable with `strconv.Atoi`
4. If `username` present, `password` required (and vice versa)

### 1.2 Link YAML Reference

**No changes** from K8s - same `settings` field:

```yaml
apiVersion: skupper.io/v2alpha1
kind: Link
metadata:
  name: link-to-remote
spec:
  endpoints:
  - host: remote-site.example.com
    port: "55671"
  tlsCredentials: link-to-remote
  settings:
    proxy-configuration: my-proxy-config    # ← References Secret name
```

### 1.3 User Workflow

1. **Create proxy Secret YAML** in `input/resources/Secret-<name>.yaml`
2. **Create/edit Link YAML** to add `settings: {proxy-configuration: <secret-name>}`
3. **Render configuration**: `skupper system reload`
4. **Verify**: Check `runtime/proxies/<name>/password.txt` exists with 0600 permissions

---

## 2. File System Layout

### 2.1 Directory Structure

```
~/.local/share/skupper/namespaces/<namespace>/
├── input/
│   └── resources/
│       ├── Secret-<proxy-name>.yaml       # User creates
│       └── Link-<link-name>.yaml          # References proxy
└── runtime/
    └── proxies/                            # System creates during render
        └── <proxy-name>/
            └── password.txt                # Only for authenticated proxies
```

### 2.2 Paths by Platform

| Platform | Host Path | Router Sees | ProfilePath Value |
|----------|-----------|-------------|-------------------|
| **Podman/Docker** | `~/.local/.../runtime/proxies/` | `/etc/skupper-router/runtime/proxies/` | `/etc/skupper-router/runtime/proxies` |
| **Linux/Systemd** | `~/.local/.../runtime/proxies/` | `~/.local/.../runtime/proxies/` | `~/.local/.../runtime/proxies` |

**Key Insight**: Containers use bind mounts; systemd uses absolute paths. The `SslProfileBasePath` (and `ProxyProfileBasePath`) differ by platform.

### 2.3 File Permissions

- `runtime/proxies/` directory: **0755**
- `runtime/proxies/<name>/` directory: **0755**
- `runtime/proxies/<name>/password.txt`: **0600** (owner read/write only)

**Rationale**: Stricter than cert files (0640) since passwords are authentication credentials.

### 2.4 Path Constants

**Already defined** in `pkg/nonkube/api/environment.go`:
```go
const (
    InputProxyProfilePath InternalPath = "input/proxies"   // Line 17 (currently unused)
    ProxyProfilesPath     InternalPath = "runtime/proxies" // Line 21
)
```

**To add** in `internal/nonkube/common/fs_config_renderer.go`:
```go
const (
    DefaultSslProfileBasePath   = "${SSL_PROFILE_BASE_PATH}"
    DefaultProxyProfileBasePath = "${SSL_PROFILE_BASE_PATH}"  // Already exists line 54
)
```

---

## 3. Code Changes

### 3.1 Site State (pkg/nonkube/api/site_state.go)

#### 3.1.1 Add getProxyConfig Method

**Location**: After `linkAccessMap()` method (~line 260)

```go
func (s *SiteState) getProxyConfig(link *v2alpha1.Link, basePath string) *site.ProxyConfig {
    proxySecretName := link.Spec.GetProxyConfiguration()
    if proxySecretName == "" {
        return nil
    }
    
    secret, ok := s.Secrets[proxySecretName]
    if !ok {
        fmt.Fprintf(os.Stderr, "Warning: proxy Secret %q not found for Link %q\n", 
            proxySecretName, link.Name)
        return nil
    }
    
    if secret.Type != "kubernetes.io/basic-auth" {
        fmt.Fprintf(os.Stderr, "Warning: Secret %q is not type kubernetes.io/basic-auth\n", 
            proxySecretName)
        return nil
    }
    
    host := string(secret.Data["host"])
    port := string(secret.Data["port"])
    if host == "" || port == "" {
        fmt.Fprintf(os.Stderr, "Warning: proxy Secret %q missing required host or port\n", 
            proxySecretName)
        return nil
    }
    
    return &site.ProxyConfig{
        Host:        host,
        Port:        port,
        User:        string(secret.Data["username"]),
        ProfilePath: path.Join(basePath, string(ProxyProfilesPath)),
    }
}
```

#### 3.1.2 Update linkMap Method

**Location**: Line 267-277

**Change**:
```go
func (s *SiteState) linkMap(sslProfileBasePath string) site.LinkMap {
    linkMap := site.LinkMap{}
    for name, link := range s.Links {
        // OLD: siteLink := site.NewLink(name, ..., &site.ProxyConfig{})
        // NEW:
        proxyConfig := s.getProxyConfig(link, sslProfileBasePath)
        siteLink := site.NewLink(
            name,
            path.Join(sslProfileBasePath, string(CertificatesPath)),
            proxyConfig,
        )
        link.SetConfigured(nil)
        siteLink.Update(link)
        linkMap[name] = siteLink
    }
    return linkMap
}
```

### 3.2 File System Config Renderer (internal/nonkube/common/fs_config_renderer.go)

#### 3.2.1 Initialize ProxyProfileBasePath

**Location**: In `Render()` method after line 78

**Add**:
```go
if c.SslProfileBasePath == "" {
    c.SslProfileBasePath = DefaultSslProfileBasePath
}
// NEW:
if c.ProxyProfileBasePath == "" {
    c.ProxyProfileBasePath = DefaultProxyProfileBasePath
}
```

#### 3.2.2 Call createProxyProfiles

**Location**: In `Render()` method after line 108 (after `createRouterConfig()`)

**Add**:
```go
err = c.createRouterConfig(siteState)
if err != nil {
    return fmt.Errorf("unable to create router config: %v", err)
}

// NEW:
err = c.createProxyProfiles(siteState)
if err != nil {
    return fmt.Errorf("unable to create proxy profiles: %v", err)
}

// Existing cert creation follows
err = c.createTlsCertificates(siteState)
```

#### 3.2.3 Add createProxyProfiles Function

**Location**: After `createTlsCertificates()` method (~line 470)

```go
func (c *FileSystemConfigurationRenderer) createProxyProfiles(siteState *api.SiteState) error {
    logger := NewLogger()
    outputPath := c.GetOutputPath(siteState)
    
    for _, link := range siteState.Links {
        proxySecretName := link.Spec.GetProxyConfiguration()
        if proxySecretName == "" {
            continue
        }
        
        secret, ok := siteState.Secrets[proxySecretName]
        if !ok {
            // Already logged warning in getProxyConfig
            continue
        }
        
        // Only write password file if credentials present
        username := secret.Data["username"]
        password := secret.Data["password"]
        if len(username) == 0 || len(password) == 0 {
            // Unauthenticated proxy - no password file needed
            continue
        }
        
        proxyDir := path.Join(outputPath, string(api.ProxyProfilesPath), proxySecretName)
        err := os.MkdirAll(proxyDir, 0755)
        if err != nil {
            return fmt.Errorf("unable to create proxy profile directory %s: %v", proxyDir, err)
        }
        
        passwordFile := path.Join(proxyDir, "password.txt")
        logger.Debug("writing proxy password", slog.String("path", passwordFile))
        err = os.WriteFile(passwordFile, password, 0600)
        if err != nil {
            return fmt.Errorf("error writing proxy password file %s: %v", passwordFile, err)
        }
    }
    
    return nil
}
```

### 3.3 Container Mounts (Compat Renderer)

**File**: `internal/nonkube/compat/site_state_renderer.go`

**Location**: In `prepareContainers()` method, line 244 (after certs mount)

**Add to FileMounts array**:
```go
FileMounts: []container.FileMount{
    {
        Source:      path.Join(siteConfigPath, string(api.RouterConfigPath)),
        Destination: "/etc/skupper-router/config",
        Options:     []string{"z"},
    },
    {
        Source:      path.Join(siteConfigPath, string(api.CertificatesPath)),
        Destination: "/etc/skupper-router/runtime/certs",
        Options:     []string{"z"},
    },
    // NEW:
    {
        Source:      path.Join(siteConfigPath, string(api.ProxyProfilesPath)),
        Destination: "/etc/skupper-router/runtime/proxies",
        Options:     []string{"z"},
    },
},
```

### 3.4 Container Mounts (Bundle Renderer)

**File**: `internal/nonkube/bundle/site_state_renderer.go`

**Location**: In container preparation, line 126 (after certs mount)

**Add to FileMounts array**:
```go
FileMounts: []container.FileMount{
    {
        Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", string(api.RouterConfigPath)),
        Destination: "/etc/skupper-router/config",
        Options:     []string{"z"},
    },
    {
        Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", string(api.CertificatesPath)),
        Destination: "/etc/skupper-router/runtime/certs",
        Options:     []string{"z"},
    },
    // NEW:
    {
        Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", string(api.ProxyProfilesPath)),
        Destination: "/etc/skupper-router/runtime/proxies",
        Options:     []string{"z"},
    },
},
```

---

## 4. Router Configuration Output

### 4.1 ProxyProfile JSON Structure

**File**: `runtime/router/skrouterd.json`

**Format** (generated by `qdr.ConfigureProxyProfile()`):

```json
{
  "proxyProfiles": [
    {
      "name": "my-proxy-config",
      "host": "proxy.example.com",
      "port": "3128",
      "username": "myuser",
      "password": "file:/etc/skupper-router/runtime/proxies/my-proxy-config/password.txt"
    }
  ],
  "connectors": [
    {
      "name": "link-to-remote",
      "proxyProfile": "my-proxy-config",
      ...
    }
  ]
}
```

**Note**: For unauthenticated proxies, `username` and `password` fields are omitted entirely.

### 4.2 Password Path Construction

**Function**: `qdr.ConfigureProxyProfile()` (already exists in `internal/qdr/qdr.go:220-230`)

**Pattern**:
```go
profile.Password = path.Join(PROXY_PATH_PREFIX, path, name, PROXY_PASSWORD_FILE)
// Expands to: "file:/etc/skupper-router/runtime/proxies/<name>/password.txt"
```

Where:
- `PROXY_PATH_PREFIX = "file:"`
- `path` = ProxyConfig.ProfilePath = `/etc/skupper-router/runtime/proxies` (podman/docker) or absolute path (systemd)
- `name` = proxy Secret name
- `PROXY_PASSWORD_FILE = "password.txt"`

---

## 5. Error Handling

### 5.1 Missing Proxy Secret

**Trigger**: Link references `proxy-configuration: <name>` but Secret doesn't exist in `SiteState.Secrets`

**Behavior**:
1. Log warning: `"Warning: proxy Secret \"<name>\" not found for Link \"<link-name>\""`
2. Create Link with `ProxyConfig = nil`
3. Router connector has no `proxyProfile` field
4. **Continue rendering** (non-fatal)

**Rationale**: Link may work without proxy if direct network access is available.

### 5.2 Malformed Proxy Secret

**Cases**:
- Wrong type (not `kubernetes.io/basic-auth`)
- Missing `host` or `port` fields
- Unparseable `port` value

**Behavior**: Same as missing Secret - log warning, create Link without proxy, continue.

### 5.3 Filesystem Errors

**Trigger**: Unable to create directory or write `password.txt`

**Behavior**: Return error from `createProxyProfiles()`, **halt rendering**

**Rationale**: Filesystem failures are system-level problems that prevent correct site configuration.

---

## 6. Platform-Specific Behavior

### 6.1 Podman/Docker

**Container launcher**: Sets `SslProfileBasePath = "/etc/skupper-router"`

**Mounts**:
- Host `runtime/proxies/` → Container `/etc/skupper-router/runtime/proxies/`
- SELinux relabel with `Options: []string{"z"}`

**Router sees**: `/etc/skupper-router/runtime/proxies/<name>/password.txt`

**ProxyProfile password field**: `"file:/etc/skupper-router/runtime/proxies/<name>/password.txt"`

### 6.2 Linux/Systemd

**Service**: Sets `SslProfileBasePath = <site-home>` (e.g., `/home/user/.local/share/skupper/namespaces/default`)

**No mounts**: Router process reads filesystem directly

**Router sees**: `/home/user/.local/share/skupper/namespaces/default/runtime/proxies/<name>/password.txt`

**ProxyProfile password field**: `"file:/home/user/.local/share/skupper/namespaces/default/runtime/proxies/<name>/password.txt"`

**Note**: Absolute paths in router config for systemd vs. container-relative paths for podman/docker.

---

## 7. Bundle Support

### 7.1 Bundle Rendering

**Proxy Secrets travel in bundles**: When a site is rendered as a bundle (`.tar.gz`), proxy Secrets are included in `input/resources/Secret-<name>.yaml` files.

**Template variables**: Bundle renderer uses `{{.NamespacesPath}}` and `{{.Namespace}}` in mount Source paths, same as certs.

**Password files**: Generated during bundle extraction/install, not included in tarball itself (security - avoid plaintext passwords in bundle file).

### 7.2 Install Process

1. Extract bundle to target host
2. Run `install.sh` script
3. Script calls rendering logic
4. `createProxyProfiles()` reads Secrets, writes password files
5. Container starts with proxy mount configured

---

## 8. Testing Strategy

### 8.1 Unit Tests

**Create**:
- `site_state_test.go`: Test `getProxyConfig()` with valid/invalid Secrets
- `fs_config_renderer_test.go`: Test `createProxyProfiles()` file creation

**Verify**:
- Correct ProxyConfig populated from Secret
- Password files created with 0600 permissions
- Warnings logged for missing Secrets
- No password file for unauthenticated proxies

### 8.2 Integration Tests

**Scenarios**:
1. **Authenticated proxy**: Secret with credentials → verify password.txt exists, router config correct
2. **Unauthenticated proxy**: Secret without credentials → verify no password.txt, router config correct
3. **Missing Secret**: Link references non-existent Secret → verify warning logged, Link created
4. **Multi-namespace**: Two namespaces with different proxies → verify isolation

**Container tests**:
- Verify mount exists: `podman inspect | grep runtime/proxies`
- Verify inside container: `podman exec ... ls /etc/skupper-router/runtime/proxies`

### 8.3 End-to-End Tests

**Setup**: Two sites behind proxy (use Squid container as test proxy)

**Steps**:
1. Create proxy Secret on linking site
2. Create Link with proxy-configuration setting
3. Render and start router
4. Verify Link becomes operational
5. Verify traffic flows through proxy (check proxy logs)

---

## 9. References

### 9.1 K8s Implementation

**Analysis document**: `.wayfinder/k8s-proxy-implementation.md`

**Key files**:
- `internal/kube/site/site.go:1312-1326` - getProxyConfig
- `internal/kube/secrets/sync.go:123-151` - handleProxyProfile
- `internal/kube/secrets/sync.go:348-361` - writeProxyProfile
- `internal/kube/adaptor/config_sync.go:107-112` - syncProxyProfileCredentialsToDisk

### 9.2 Examples

**User-facing examples**: `.wayfinder/proxy-examples.md`

**Covers**:
- Authenticated/unauthenticated proxies
- Multi-namespace scenarios
- Error cases
- Platform differences
- Verification commands

### 9.3 Design Decisions

**Wayfinding tickets**: `.wayfinder/tickets/`
- 001: K8s implementation analysis
- 002: Secret YAML schema
- 003: File system layout
- 004: Router config rendering
- 005: Container volume mounts
- 006: Example configurations

---

## 10. Implementation Checklist

### Phase 1: Core Rendering (Required)
- [ ] Add `getProxyConfig()` method to `pkg/nonkube/api/site_state.go`
- [ ] Update `linkMap()` to call `getProxyConfig()`
- [ ] Add `ProxyProfileBasePath` initialization in `fs_config_renderer.go`
- [ ] Add `createProxyProfiles()` function
- [ ] Call `createProxyProfiles()` from `Render()`

### Phase 2: Container Support (Required)
- [ ] Add proxy mount to `internal/nonkube/compat/site_state_renderer.go`
- [ ] Add proxy mount to `internal/nonkube/bundle/site_state_renderer.go`

### Phase 3: Testing (Required)
- [ ] Unit tests for `getProxyConfig()`
- [ ] Unit tests for `createProxyProfiles()`
- [ ] Integration test: authenticated proxy
- [ ] Integration test: unauthenticated proxy
- [ ] Integration test: missing Secret warning
- [ ] End-to-end test with real proxy

### Phase 4: Documentation (Recommended)
- [ ] User docs: How to configure proxy for Links
- [ ] Copy examples from `.wayfinder/proxy-examples.md` to docs
- [ ] Troubleshooting guide for proxy issues
- [ ] Update architecture docs with proxy flow

---

## 11. Out of Scope (Future Work)

The following are **explicitly out of scope** for this design:

1. **CLI helper for Secret creation**: No `skupper secret create` command. Users manually create Secret YAML.
2. **Runtime credential rotation**: Changes to proxy Secrets require `skupper system reload`. No dynamic reconciliation.
3. **Proxy for RouterAccess**: Only outbound Links support proxy. Inbound RouterAccess connections don't traverse proxies.
4. **Migration from custom workarounds**: This is a new feature, not a replacement.
5. **Deep validation**: No connectivity checks or credential validation at config time. Invalid proxy fails at runtime.

---

## 12. Summary

This design extends HTTP proxy support to non-kube sites by:

1. **Reusing K8s API**: Same Link YAML `settings` pattern, same Secret format
2. **File-based storage**: Secrets as YAML files instead of K8s API objects
3. **Mirroring file patterns**: Proxy profiles follow cert pattern (`runtime/proxies/` like `runtime/certs/`)
4. **Platform adaptation**: Container mounts for podman/docker, absolute paths for systemd
5. **Conservative error handling**: Log warnings, continue rendering (fail fast only on filesystem errors)

**Code changes**: ~150 lines across 4 files (2 new methods, 1 new function, 2 mount additions).

**Estimated effort**: 2-3 days implementation + 1-2 days testing.

**Risk**: Low - pattern proven in K8s, minimal API surface, clear error boundaries.
