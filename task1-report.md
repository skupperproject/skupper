# Task 1 Report: Trace how a Link becomes router config today (without proxy)

## Data flow summary

```
SiteState.ToRouterConfig()
  └─ SiteState.linkMap(sslProfileBasePath)        # site_state.go:267
       └─ site.NewLink(name, certPath, &ProxyConfig{})  # empty proxy config
            └─ LinkMap.Apply(&routerConfig)         # site_state.go:335
                 └─ Link.Apply(current)             # link.go:41
                      ├─ proxyProfileName = GetProxyConfiguration()  → ""
                      ├─ current.AddConnector(connector)   # ProxyProfile field = ""
                      ├─ current.AddSslProfile(...)
                      └─ if proxyProfileName != "" → FALSE, skipped
```

## Step-by-step trace

### 1. `SiteState.linkMap()` — `pkg/nonkube/api/site_state.go:267`

Iterates `s.Links` and creates a `site.Link` for each one:

```go
siteLink := site.NewLink(name, path.Join(sslProfileBasePath, string(CertificatesPath)), &site.ProxyConfig{})
```

The `TODO: proxy profile config ?` comment at line 270 marks the insertion point for future proxy resolution. Currently an empty `&site.ProxyConfig{}` (all zero-value fields) is passed for every link.

### 2. `site.NewLink()` — `internal/site/link.go:25`

Stores the empty `proxyConfig` on the `Link` struct:

```go
type ProxyConfig struct {
    Host        string   // ""
    Port        string   // ""
    User        string   // ""
    ProfilePath string   // ""
}
```

### 3. `Link.Apply()` — `internal/site/link.go:41`

This is the key branching point. It calls:

```go
proxyProfileName := proxyProfileName(l.definition)
```

Which delegates to `LinkSpec.GetProxyConfiguration()` (`pkg/apis/skupper/v2alpha1/types.go:591`):

```go
func (s *LinkSpec) GetProxyConfiguration() string {
    if value, ok := s.Settings["proxy-configuration"]; ok {
        return value
    }
    return ""
}
```

For a link with no proxy, `Settings` has no `"proxy-configuration"` key, so this returns `""`.

Back in `Apply()`, the guard at line 71:

```go
if proxyProfileName != "" {
    current.AddProxyProfile(qdr.ConfigureProxyProfile(...))
}
```

evaluates to `false`. No `ProxyProfile` is added to the router config.

The `Connector` is still added (line 69), but its `ProxyProfile` field is `""`:

```go
connector := qdr.Connector{
    Name:         l.name,
    ProxyProfile: proxyProfileName,  // ""
    ...
}
```

### 4. `ToRouterConfig()` — `pkg/nonkube/api/site_state.go:296`

Calls `s.linkMap(sslProfileBasePath).Apply(&routerConfig)` at line 335. After `Apply` returns, `routerConfig.ProxyProfiles` remains the empty map initialized in `InitialConfig()` (`internal/qdr/qdr.go:57`):

```go
ProxyProfiles: map[string]ProxyProfile{},
```

### 5. `MarshalRouterConfig()` — `internal/qdr/qdr.go:900`

Iterates `config.ProxyProfiles` at line 914:

```go
for _, e := range config.ProxyProfiles {
    tuple := []interface{}{
        "proxyProfile",
        e,
    }
    elements = append(elements, tuple)
}
```

With an empty map, this loop body never executes. No `proxyProfile` tuples appear in the marshaled JSON.

The `Connector` tuple is emitted, but its `proxyProfile` field serializes as `""` and is omitted from JSON output by the `omitempty` tag on `Connector.ProxyProfile`.

## Verification

**Confirmed**: for a Link with no `proxy-configuration` setting, the generated router config JSON contains zero `proxyProfile` entries. The `Connector` element references no proxy profile. The `ProxyProfiles` map on `RouterConfig` stays empty from initialization through marshaling.

## Key observations for proxy implementation

1. **Insertion point**: `linkMap()` at `site_state.go:271` is where proxy resolution logic must be added. The empty `&site.ProxyConfig{}` needs to be replaced with a resolved config when the Link's `Settings["proxy-configuration"]` references a Secret.

2. **Shared code**: `Link.Apply()` and `ConfigureProxyProfile()` are platform-agnostic. The non-kube implementation only needs to provide a populated `ProxyConfig` struct — the wiring into router config is already handled.

3. **`file:` prefix convention**: `ConfigureProxyProfile()` (`qdr.go:228`) constructs the password path as `file:<basePath>/<profileName>/password.txt`. The non-kube renderer must write password files to a directory structure matching this convention.

4. **Guard behavior**: The `proxyProfileName != ""` check means passing an empty `ProxyConfig` is safe — it's a no-op. The implementation only activates when the Link spec actually declares a proxy configuration.
