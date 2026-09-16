---
id: 004
title: Router Config Rendering Design
type: grilling
status: closed
blocks: [003]
created: 2026-09-15
resolved: 2026-09-15
---

## Question

How should `fs_config_renderer.go` and `site_state.go` be modified to render proxy profiles? Design the code changes needed to transform proxy Secret YAML into router config JSON and password files.

**Areas to cover**:

1. **Secret loading**: Where/how to read proxy Secret YAML files during rendering (in `Render()` or `createRouterConfig()`?)
2. **Password file writing**: Where to write `password.txt` - new function or extend existing cert-writing logic?
3. **ProxyConfig population**: How to replace the TODO at `site_state.go:270` - pass ProxyConfig to `NewLink()` instead of empty struct
4. **Path variable substitution**: K8s uses `ProfilePath` - do we need template variables for bundles like certs do?
5. **Error handling**: What happens if Link references non-existent proxy Secret? Log and skip? Fail rendering?
6. **Testing strategy**: How to verify proxy config rendering without full router integration?

**Constraints**:
- Must use the same `qdr.ConfigureProxyProfile()` function K8s uses
- Password path must follow ticket #003's design
- Keep rendering deterministic and idempotent

**Dependencies**: Blocked by #003 (file system layout) - need to know paths before implementing rendering.

**Why**: This is the core transformation logic - bugs here break proxy functionality silently.

**How to apply**: Implementation guide for the engineer writing the rendering code.

---

## Resolution

### 1. Password File Writing Location

**Add new function `createProxyProfiles()` in `fs_config_renderer.go`** called after `createRouterConfig()` at line 108-109:

```go
// Line 105-120 becomes:
err = c.createRouterConfig(siteState)
if err != nil {
    return fmt.Errorf("unable to create router config: %v", err)
}

// NEW: Write proxy profile password files
err = c.createProxyProfiles(siteState)
if err != nil {
    return fmt.Errorf("unable to create proxy profiles: %v", err)
}

// Creating the certificates
err = c.createTlsCertificates(siteState)
```

This mirrors the cert pattern and keeps rendering organized.

### 2. ProxyConfig Population in site_state.go

**Add method to SiteState** (in `pkg/nonkube/api/site_state.go`):

```go
func (s *SiteState) getProxyConfig(link *v2alpha1.Link, basePath string) *site.ProxyConfig {
    proxySecretName := link.Spec.GetProxyConfiguration()
    if proxySecretName == "" {
        return nil
    }
    
    secret, ok := s.Secrets[proxySecretName]
    if !ok {
        // Log warning but don't fail - missing proxy shouldn't block rendering
        fmt.Fprintf(os.Stderr, "Warning: proxy Secret %q not found for Link %q\n", 
            proxySecretName, link.Name)
        return nil
    }
    
    // Validate it's a basic-auth secret
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

**Update linkMap()** at line 267-277:

```go
func (s *SiteState) linkMap(sslProfileBasePath string) site.LinkMap {
    linkMap := site.LinkMap{}
    for name, link := range s.Links {
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

### 3. ProxyProfileBasePath Initialization

**Add to `Render()` in `fs_config_renderer.go`** at line 76-81:

```go
if c.SslProfileBasePath == "" {
    c.SslProfileBasePath = DefaultSslProfileBasePath
}
if c.ProxyProfileBasePath == "" {
    c.ProxyProfileBasePath = DefaultProxyProfileBasePath
}
```

Keep existing constant (line 54): `DefaultProxyProfileBasePath = "${SSL_PROFILE_BASE_PATH}"` since proxy path is relative to same base as certs.

### 4. Password File Writing Function

**Add to `fs_config_renderer.go`** (after `createTlsCertificates` around line 470):

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
            // Already logged in getProxyConfig
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

### 5. Error Handling Strategy

**Missing/malformed proxy Secret**: Log warning, create Link without proxy (ProxyConfig = nil). Don't fail rendering - the Link may work without proxy if direct network access is available.

**Filesystem errors**: Return error from `createProxyProfiles()` - these are system failures that should halt rendering.

**Validation errors** (in getProxyConfig):
- Missing Secret: log warning, return nil
- Wrong type: log warning, return nil
- Missing host/port: log warning, return nil
- Missing credentials: valid for unauthenticated proxy, return ProxyConfig without User field

### 6. Bundle Template Variables

No special handling needed. The ProxyProfileBasePath uses same template variable as SSL: `${SSL_PROFILE_BASE_PATH}`. Bundle rendering will substitute it just like it does for cert paths.

### Summary of Changes

**Files to modify**:
1. `pkg/nonkube/api/site_state.go`:
   - Add `getProxyConfig()` method (~30 lines)
   - Update `linkMap()` to call it (replace line 270-271)

2. `internal/nonkube/common/fs_config_renderer.go`:
   - Add ProxyProfileBasePath initialization in `Render()` (2 lines at ~line 79)
   - Add call to `createProxyProfiles()` in `Render()` (4 lines at ~line 108)
   - Add `createProxyProfiles()` function (~40 lines after createTlsCertificates)

**No changes needed**:
- Constants already defined in `environment.go`
- ProxyConfig struct already exists in `internal/site/link.go`
- qdr.ConfigureProxyProfile already exists
