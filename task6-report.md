# Task 6 Report: Compare K8s proxy flow (read-only, for context)

## Data flow summary

```
K8s Secret watch (informer)
  └─ Sync.handle()                           # sync.go:153
       └─ secret.Type == "kubernetes.io/basic-auth"
            └─ Sync.handleProxyProfile()      # sync.go:123
                 ├─ getConfiguredProxy()       # check if router expects this profile
                 ├─ updateSecretChecksum()     # SHA-256 change detection
                 ├─ writeProxyProfile()        # sync.go:348 — write password.txt (0644)
                 └─ doCallback()              # triggers reconcileAfterTlsSecretChange
                      └─ updateRouterConfigForGroup()  # AMQP push to router

Site.link()                                   # site.go:1348
  └─ Site.newLink()                           # site.go:1328
       └─ Site.getProxyConfig()               # site.go:1312
            ├─ profiles.Cache.Get(secret)     # hard error if missing
            └─ returns &ProxyConfig{Host, Port, User, ProfilePath}
                 └─ site.NewLink(name, SSL_PROFILE_PATH, proxyConfig)
                      └─ Link.Apply() → AddProxyProfile()  # shared code
```

## Step-by-step trace

### 1. `Site.getProxyConfig()` — `internal/kube/site/site.go:1312`

Looks up the proxy Secret by name from the informer cache:

```go
proxySecret, err := s.profiles.Cache.Get(s.namespace + "/" + proxySetting)
```

If the Secret is missing, returns a **hard error**: `"Secret not found for proxy configuration"`. This error propagates up through `newLink()` (line 1332) and causes the Link to fail with a status condition update (line 1362).

Contrast with non-kube: the plan-review recommends logging a warning and returning `nil`, allowing the Link to proceed without a proxy. This divergence is deliberate — K8s has dynamic reconciliation so the Secret can appear later and the Link will retry, whereas non-kube renders once.

When the Secret exists, it reads fields directly from `secret.Data`:

```go
return &site.ProxyConfig{
    Host:        string(proxySecret.Data["host"]),
    Port:        string(proxySecret.Data["port"]),
    User:        string(proxySecret.Data["username"]),
    ProfilePath: PROXY_PROFILE_PATH,   // "/etc/skupper-router-proxies"
}
```

### 2. `Site.newLink()` — `internal/kube/site/site.go:1328`

Calls `getProxyConfig()`, then passes the result into the shared `site.NewLink()`:

```go
config := site.NewLink(linkconfig.ObjectMeta.Name, SSL_PROFILE_PATH, proxyConfig)
```

From this point, `Link.Apply()` handles profile wiring identically to non-kube (see Task 1). The path constants differ:
- K8s: `SSL_PROFILE_PATH = "/etc/skupper-router-certs"`, `PROXY_PROFILE_PATH = "/etc/skupper-router-proxies"`
- Non-kube (container): resolved via `${SSL_PROFILE_BASE_PATH}` env var → `/etc/skupper-router`
- Non-kube (systemd): absolute host path like `/home/user/.local/share/skupper/namespaces/default`

### 3. `writeProxyProfile()` — `internal/kube/secrets/sync.go:348`

Writes the password file to disk:

```go
func writeProxyProfile(secret *corev1.Secret, filePath string) error {
    _, ok := secret.Data["password"]
    if !ok {
        return fmt.Errorf("empty proxyProfile %q", secret.Name)
    }
    baseName := path.Dir(filePath)
    os.MkdirAll(baseName, 0755)           // directory permissions
    writeFile(filePath, secret.Data["password"], 0644)  // file permissions
}
```

The `filePath` comes from the configured `ProxyProfile.Password` field with the `file:` prefix stripped (line 142):

```go
path := strings.TrimPrefix(proxyProfile.Password, "file:")
```

This constructs a path like `/etc/skupper-router-proxies/<profileName>/password.txt`.

**Permission difference**: K8s uses `0644` for the password file. The non-kube plan specifies `0600`. The plan-review (suggestion 7) flags this as intentional but notes the router process must be able to read the file.

### 4. `Sync.handleProxyProfile()` — `internal/kube/secrets/sync.go:123`

The dynamic-watch handler. Triggered by the K8s informer when a `kubernetes.io/basic-auth` Secret is created, updated, or re-synced.

Key steps:

1. **Profile lookup**: `getConfiguredProxy(profileName)` checks if the router config currently expects this profile. If not (`!isConfigured`), the Secret is cached but no file is written (line 133-135).

2. **Checksum-based change detection**: `updateSecretChecksum(secret, &prev.SecretContentSum)` computes a SHA-256 over the Secret's Data fields (sorted keys, length-prefixed values). Only proceeds to write if the checksum changed or this is the first time seeing the Secret (line 140).

3. **Conditional write**: Only writes the password file if both `username` and `password` are present in the Secret data (line 141).

4. **Callback chain**: If a write occurred, `doCallback(secret.Name)` triggers `reconcileAfterTlsSecretChange()` (site.go:871), which reapplies the desired router config across all router groups via AMQP.

### 5. `Sync.ExpectProxyProfiles()` — `internal/kube/secrets/sync.go:260`

Called when the router config is updated with new proxy profile expectations. It:
- Stores the expected profiles via `setConfiguredProxy()`
- For each expected profile, looks up the Secret from cache
- If the Secret exists, calls `handleProxyProfile()` to write the password file
- Enriches the profile with host/port/username from the Secret data (lines 278-281)
- Returns a `SyncDelta` with missing Secrets and proxy updates

### 6. Checksum and ordinal machinery — `internal/kube/secrets/context.go`

**`updateSecretChecksum()`** (context.go:113): SHA-256 hash of all Data fields, sorted by key, length-prefixed. Used to detect Secret content changes without comparing raw bytes.

**`profileContext`** (context.go:36): Contains `ProfileName` and `Ordinal`. Ordinals are used for SSL profiles to track version ordering — ensuring a newer Secret version isn't overwritten by a stale one during concurrent updates. **Proxy profiles don't use ordinals** — the `handleProxyProfile` function uses only `SecretContentSum` for change detection.

**`IsTlsCredentialSecret()`** (context.go:23): Discriminates Secret types. Explicitly excludes `SecretTypeBasicAuth` (line 24), routing those to the proxy handler instead.

## Verification

**Confirmed**: the K8s flow is more complex for three reasons the non-kube implementation does not need:

1. **Dynamic Secret watching**: K8s uses informer-based watches that fire `handle()` whenever a Secret changes. Non-kube reads Secrets once from YAML files at render time.

2. **AMQP router sync**: After writing files, K8s pushes updated config to the live router via `reconcileAfterTlsSecretChange()` → `updateRouterConfigForGroup()`. Non-kube generates a static config file; the router reads it at startup and requires explicit reload for changes.

3. **Checksum-based deduplication**: K8s must avoid redundant writes and AMQP pushes during informer re-syncs, so it tracks SHA-256 checksums per Secret. Non-kube renders once — there's no re-sync to deduplicate.

Ordinal tracking (used for SSL profiles to handle version conflicts during concurrent updates) is **not** used for proxy profiles even in K8s — `handleProxyProfile` relies solely on checksum comparison.

## Key observations for non-kube proxy implementation

1. **Hard error vs. warning**: K8s `getProxyConfig()` returns a hard error on missing Secret (line 1322). Non-kube should log a warning and return `nil` per the plan-review, since there's no dynamic reconciliation to retry later.

2. **Permissions**: K8s writes password files with `0644` (sync.go:357). Non-kube plan specifies `0600`. Verify the router process can read the file with tighter permissions.

3. **`file:` prefix**: Both platforms use `ConfigureProxyProfile()` from `internal/qdr/qdr.go:220`, which constructs `file:<basePath>/<name>/password.txt`. The non-kube renderer must write files to match this path convention.

4. **No checksum/ordinal/callback needed**: The non-kube flow writes files once during rendering. No change detection, no AMQP push, no callback chain. `createProxyProfiles()` in `fs_config_renderer.go` is a straightforward write loop analogous to `createTlsCertificates()`.

5. **Secret type discrimination**: K8s uses `secret.Type == "kubernetes.io/basic-auth"` (sync.go:177) to route Secrets to the proxy handler. Non-kube should use the same discriminator to identify proxy Secrets in `SiteState.Secrets`.

6. **Path constants differ by platform**: K8s uses hardcoded `/etc/skupper-router-proxies`. Non-kube containers use `${SSL_PROFILE_BASE_PATH}`-relative paths. Non-kube systemd uses absolute host paths. The shared `ConfigureProxyProfile()` accepts `path` as a parameter, so the implementation just needs to pass the right base path.
