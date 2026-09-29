# Task 3 Report: Understand the certificate-writing pattern you'll mirror

## Render pipeline overview

```
FileSystemConfigurationRenderer.Render(siteState)             # fs_config_renderer.go:72
  ├─ 1. Create directories (configurationDirectories loop)    # line 97
  │     └─ os.MkdirAll(dir, 0755) for each InternalPath
  ├─ 2. createRouterConfig(siteState)                         # line 105
  │     └─ ToRouterConfig() → MarshalRouterConfig() → WriteFile("skrouterd.json", 0644)
  ├─ 3. createTlsCertificates(siteState)                      # line 111
  │     ├─ Phase A: Generate/write CA certs → runtime/issuers/<name>/
  │     ├─ Phase B: Generate/write client/server certs → runtime/certs/<name>/
  │     └─ Phase C: Write link certs → runtime/certs/<secretName>-profile/
  └─ 4. createTokens(siteState)                               # line 117
        └─ Write token YAMLs → runtime/links/
```

## Step-by-step trace

### 1. Directory pre-creation — `fs_config_renderer.go:97`

The `configurationDirectories` slice (line 22) lists paths created at render time:

```go
configurationDirectories = []api.InternalPath{
    api.RouterConfigPath,      // "runtime/router"
    api.IssuersPath,           // "runtime/issuers"
    api.CertificatesPath,      // "runtime/certs"       ← present
    api.InputIssuersPath,      // "input/issuers"
    api.InputCertificatesPath, // "input/certs"
    api.InputSiteStatePath,    // "input/resources"
    api.LoadedSiteStatePath,   // "internal/snapshot"
    api.RuntimeSiteStatePath,  // "runtime/resources"
    api.RuntimeTokenPath,      // "runtime/links"
    api.ScriptsPath,           // "internal/scripts"
}
```

**`api.ProxyProfilesPath` (`"runtime/proxies"`) is NOT in this list.** The directory is defined in `environment.go:21` but never pre-created.

Each directory is created with `os.MkdirAll(configDir, 0755)` (line 99).

### 2. `createRouterConfig()` — `fs_config_renderer.go:317`

Generates router config via `siteState.ToRouterConfig()`, marshals to JSON, writes to `runtime/router/skrouterd.json` with permission `0644`.

This step runs **before** certificate writing. The router config references cert/proxy paths but does not depend on the files existing yet — it just embeds the path strings.

### 3. `createTlsCertificates()` — `fs_config_renderer.go:335`

This is the pattern that `createProxyProfiles()` will mirror.

#### Inner helper: `writeSecretFilesIgnore()` (line 338)

```
writeSecretFilesIgnore(basePath, secret, ignoreExisting)
  ├─ os.Open(basePath)
  │   ├─ not exist → os.MkdirAll(basePath, 0755)     # create subdirectory
  │   └─ exists → verify is directory
  ├─ for fileName, data := range secret.Data:
  │     ├─ if file exists && ignoreExisting → skip (warn)
  │     └─ os.WriteFile(path.Join(basePath, fileName), data, 0640)
  └─ return nil
```

Key details:
- **Directory permission**: `0755` (line 342)
- **File permission**: `0640` (line 368) — group-readable, not world-readable
- **Idempotency**: optional `ignoreExisting` flag skips overwrite for CA certs
- **Error handling**: hard errors on directory/write failures, returns `error`

#### Phase A: CA certificates (line 378-408)

Iterates `siteState.Certificates` where `Signing == true`. Loads or generates a CA secret, writes to `runtime/issuers/<name>/`. Uses `ignoreExisting: true` so user-provided CAs aren't overwritten.

#### Phase B: Client/server certificates (line 410-458)

Iterates `siteState.Certificates` for client/server certs. Signs with CA, writes to `runtime/certs/<name>/`. Uses `ignoreExisting: false` (always overwrites).

#### Phase C: Link certificates (line 460-472)

This is the most directly relevant phase for proxy profiles:

```go
for _, link := range siteState.Links {
    secretName := link.Spec.TlsCredentials
    secret, ok := siteState.Secrets[secretName]
    if !ok {
        return fmt.Errorf("secret %s not found", secretName)   // hard error
    }
    certPath := path.Join(outputPath, string(api.CertificatesPath), secretName+"-profile")
    err = writeSecretFiles(certPath, secret)
}
```

For each Link:
1. Look up the Secret by the link's `TlsCredentials` name
2. Hard error if Secret not found
3. Write all Secret data files into `runtime/certs/<secretName>-profile/`

**This is the exact pattern proxy profiles will follow**, but writing to `runtime/proxies/<proxyName>/` and using only the `password` field.

### 4. `createTokens()` — `fs_config_renderer.go:273`

Writes token YAML files to `runtime/links/` with permission `0644`. Not directly relevant to proxy profiles but completes the render ordering picture.

## Reload behavior — `fs_config_renderer.go:236`

The `reloadDirectories` slice (line 34) lists directories cleaned on reload:

```go
reloadDirectories = []api.InternalPath{
    api.RouterConfigPath,      // "runtime/router"
    api.CertificatesPath,      // "runtime/certs"       ← present
    api.RuntimeSiteStatePath,  // "runtime/resources"
    api.RuntimeTokenPath,      // "runtime/links"
    api.ScriptsPath,           // "internal/scripts"
    api.LoadedSiteStatePath,   // "internal/snapshot"
}
```

**`api.ProxyProfilesPath` is also missing from `reloadDirectories`.** On reload, `runtime/proxies/` would survive cleanup — stale proxy passwords could persist.

## Verification: `runtime/certs/` directory structure for a site with a Link

Given a site with:
- A CA cert named `skupper-site-ca`
- A server cert named `skupper-site-server` signed by that CA
- A client cert named `client-skupper-site-server`
- A Link with `tlsCredentials: "link-token-abc"`

The rendered structure would be:

```
runtime/
├── certs/                              # pre-created by configurationDirectories
│   ├── skupper-site-server/            # server cert directory
│   │   ├── tls.crt                     # 0640
│   │   ├── tls.key                     # 0640
│   │   └── ca.crt                      # 0640
│   ├── client-skupper-site-server/     # client cert directory
│   │   ├── tls.crt                     # 0640
│   │   ├── tls.key                     # 0640
│   │   ├── ca.crt                      # 0640
│   │   └── connect.json               # 0640
│   └── link-token-abc-profile/         # link cert directory (Phase C)
│       ├── tls.crt                     # 0640
│       ├── tls.key                     # 0640
│       └── ca.crt                      # 0640
├── issuers/
│   └── skupper-site-ca/
│       ├── tls.crt                     # 0640
│       └── tls.key                     # 0640
├── router/
│   └── skrouterd.json                  # 0644
└── links/
    └── <token>.yaml                    # 0644
```

All cert files use `0640`. Directories use `0755`. The parent `runtime/certs/` directory is pre-created; subdirectories are created on-demand by `writeSecretFilesIgnore`.

## Key observations for proxy implementation

1. **Mirror Phase C, not Phase A/B.** The proxy profile writer should iterate `siteState.Links`, look up proxy Secrets, and write password files — same structure as the link certificate loop (lines 460-472). No cert generation or CA signing needed.

2. **Add `api.ProxyProfilesPath` to both directory slices.** The path constant exists (`environment.go:21`) but is missing from both `configurationDirectories` (line 22) and `reloadDirectories` (line 34). Add it to both for consistency.

3. **Permission divergence is intentional.** Cert files use `0640`. The plan specifies `0600` for proxy passwords. This is more restrictive than certs, which is reasonable since password files contain plaintext credentials rather than TLS key material (which is already protected by the key pair model). Verify the router process can read `0600` files.

4. **Error handling divergence from K8s.** Phase C uses a hard error for missing link secrets (`return fmt.Errorf`). The plan's `createProxyProfiles()` should use a warning-and-continue approach (as noted in plan-review.md suggestion 6) since non-kube sites lack dynamic reconciliation.

5. **Render ordering matters.** `createRouterConfig()` runs before certificate/proxy writing. The router config embeds `file:` path references that must match where files are actually written. Both use the same base path constants, so they stay in sync.

6. **`writeSecretFilesIgnore` could be reused** for proxy profiles, but the plan calls for writing only the `password` field with different permissions (`0600` vs `0640`). A dedicated helper is cleaner than adding permission parameters to the existing function.
