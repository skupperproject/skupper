# Spec: HTTP Proxy Support for Non-Kube Sites

**Status**: Ready for implementation
**Date**: 2026-09-29

---

## Problem Statement

Skupper Links on Kubernetes sites can route through HTTP proxies by referencing a proxy Secret in the Link's `settings.proxy-configuration` field. This capability does not exist for non-Kubernetes sites (podman, docker, linux/systemd). Users operating in corporate or air-gapped environments where outbound connections must traverse an HTTP proxy cannot establish inter-site Links from local system sites.

## Solution

Extend the non-kube site rendering pipeline to read proxy configuration from file-based Secrets, write proxy password files to disk, generate ProxyProfile entries in router config JSON, and mount the proxy directory into router containers. The user-facing API (Link YAML `settings.proxy-configuration` referencing a Secret by name) is identical to Kubernetes — only the backing storage changes from the K8s Secret API to file-based Secret YAML.

## User Stories

1. As a site operator behind a corporate HTTP proxy, I want to configure a Link to route through my authenticated proxy, so that my non-kube site can connect to remote sites without direct network access.
2. As a site operator behind an unauthenticated proxy, I want to configure a Link with only host and port (no credentials), so that I can use transparent proxies that don't require authentication.
3. As a site operator running podman/docker sites, I want proxy password files mounted into the router container automatically, so that the router can read credentials at runtime without manual volume configuration.
4. As a site operator running a linux/systemd site, I want the router to read proxy passwords directly from the filesystem using absolute paths, so that proxy support works without container mount indirection.
5. As a site operator managing multiple namespaces, I want each namespace's proxy configuration isolated, so that different sites can use different proxies without interference.
6. As a site operator, I want clear warning messages when a Link references a missing or malformed proxy Secret, so that I can diagnose configuration errors without reading source code.
7. As a site operator, I want site rendering to continue even when a proxy Secret is missing, so that a misconfigured proxy doesn't block my entire site from starting.
8. As a site operator deploying via bundles, I want proxy Secrets included in the bundle and password files generated at install time, so that proxied Links work on airgapped target hosts.
9. As a site operator, I want proxy password files written with 0600 permissions, so that credentials are protected from other users on the system.
10. As a site operator, I want to update proxy credentials by editing the Secret YAML and running `skupper system reload`, so that credential rotation follows the same workflow as other configuration changes.
11. As a site operator, I want the proxy Secret format to match Kubernetes (`type: kubernetes.io/basic-auth` with `host`, `port`, `username`, `password` fields), so that I can use the same documentation and mental model across platforms.
12. As a site operator, I want proxy configuration to appear in the router config JSON as ProxyProfile entries referenced by connectors, so that the standard router proxy mechanism handles the actual proxying.

## Implementation Decisions

### Modules Modified

- **SiteState** (`pkg/nonkube/api/`) — Add `getProxyConfig()` method to extract proxy configuration from Secrets; update `linkMap()` to pass populated ProxyConfig to `site.NewLink()` instead of the current empty struct (replacing the TODO at the existing call site).
- **FileSystemConfigurationRenderer** (`internal/nonkube/common/`) — Add `createProxyProfiles()` function to write password files; call it from `Render()` between `createRouterConfig()` and `createTlsCertificates()`.
- **Compat SiteStateRenderer** (`internal/nonkube/compat/`) — Add FileMount for `runtime/proxies` → `/etc/skupper-router/runtime/proxies` in `prepareContainers()`.
- **Bundle SiteStateRenderer** (`internal/nonkube/bundle/`) — Same mount addition using template variables.
- **Linux SiteStateRenderer** (`internal/nonkube/linux/`) — Ensure `ProxyProfileBasePath` is set to the site home directory (no container mount needed).

### Secret Schema

Proxy Secrets use native Kubernetes Secret format stored at `input/resources/Secret-<name>.yaml`:
- Type discriminator: `type: kubernetes.io/basic-auth`
- Required fields: `host` (string), `port` (string, parseable as integer)
- Optional fields: `username` and `password` (must pair together)
- Unauthenticated proxies omit `username` and `password` entirely

### File System Layout

Password files written at render time:
- Path: `runtime/proxies/<secret-name>/password.txt` (permissions 0600)
- Directory: `runtime/proxies/<secret-name>/` (permissions 0755)
- Path constants `InputProxyProfilePath` and `ProxyProfilesPath` already exist in `environment.go`

### Router Config Output

ProxyProfile entries generated via the existing `qdr.ConfigureProxyProfile()` function:
- Password referenced as `file:<base-path>/runtime/proxies/<name>/password.txt`
- Connectors reference ProxyProfiles by name via the `proxyProfile` field
- Unauthenticated proxies: ProxyProfile has host and port only, no username/password fields

### Platform Path Differences

- **Podman/Docker**: ProxyProfileBasePath = `/etc/skupper-router/runtime/proxies` (via container bind mount)
- **Linux/Systemd**: ProxyProfileBasePath = `<site-home>/runtime/proxies` (absolute filesystem path)
- The ProxyProfileBasePath follows the same pattern as SslProfileBasePath — it's set by each platform's renderer

### Error Handling

- Missing proxy Secret: log warning to stderr, create Link without proxy (non-fatal)
- Wrong Secret type (not `kubernetes.io/basic-auth`): log warning, skip proxy (non-fatal)
- Missing host or port fields: log warning, skip proxy (non-fatal)
- Filesystem errors (can't create directory or write file): return error, halt rendering (fatal)

### Secret Loading

No new loading mechanism needed. Secrets are already loaded by `FileSystemSiteStateLoader` from `input/resources/Secret-*.yaml` files into `SiteState.Secrets` map. Proxy Secrets are discovered by their `type` field during `getProxyConfig()`, not by filename convention.

## Testing Decisions

### What Makes a Good Test

Tests should exercise the rendering pipeline through its public interface (`Render()`), asserting on observable outputs (filesystem artifacts and router config JSON content). Tests should not test internal method signatures, struct field values, or intermediate state — only the externally visible result of a render pass.

### Testing Seam

**Single seam: `FileSystemConfigurationRenderer.Render()`**

The existing test in `fs_config_renderer_test.go` uses a `fakeSiteState()` fixture, calls `Render()` on a temp directory, and asserts that expected files exist with correct content. Proxy tests extend this same pattern:

- Add a proxy Secret (type `kubernetes.io/basic-auth` with host, port, username, password) to `fakeSiteState()`
- Add `Settings: map[string]string{"proxy-configuration": "<secret-name>"}` to the Link fixture
- After `Render()`, assert:
  - `runtime/proxies/<name>/password.txt` exists with correct content and 0600 permissions
  - `runtime/router/skrouterd.json` contains a ProxyProfile entry with correct host, port, username, and password file reference
  - The connector for the link references the ProxyProfile by name

### Test Cases

1. **Authenticated proxy**: Link with proxy-configuration setting, Secret with credentials → password file written, ProxyProfile in router config
2. **Unauthenticated proxy**: Secret with only host/port → no password file, ProxyProfile with only host/port in router config
3. **Missing Secret**: Link references non-existent Secret → no ProxyProfile, warning logged, render succeeds
4. **No proxy configured**: Link without settings field → no proxy artifacts, no ProxyProfile (regression guard)

### Prior Art

- `internal/nonkube/common/fs_config_renderer_test.go` — `TestFileSystemConfigurationRenderer_Render` with `fakeSiteState()` pattern
- `internal/kube/secrets/sync_test.go` — `TestSyncExpectProxy` with `fixtureProxySecret()` and `fixtureProxyProfile()` for K8s-side reference

## Out of Scope

- **CLI helper for Secret creation**: No `skupper secret create` command. Users create Secret YAML files manually.
- **Runtime credential rotation**: Proxy Secret changes require `skupper system reload`. No file-watch or dynamic reconciliation.
- **Proxy for RouterAccess**: Only outbound Links support proxy. Inbound RouterAccess connections don't traverse proxies.
- **Deep validation**: No proxy connectivity checks or credential verification at config time. Invalid proxy config fails at runtime.
- **Migration tooling**: This is a new feature — no existing proxy workarounds to migrate from.
- **Site state validator changes**: Proxy Secret validation (type, required fields) happens at render time in `getProxyConfig()`, not in the validator. The validator doesn't need changes for MVP.

## Further Notes

- The implementation is approximately 150 lines across 4–5 files: 2 new methods on SiteState, 1 new function on FileSystemConfigurationRenderer, and 2 mount additions in compat/bundle renderers.
- The K8s implementation reference (with exact function signatures and data flows) is documented at `.wayfinder/k8s-proxy-implementation.md`.
- Complete user-facing examples (authenticated, unauthenticated, multi-namespace, error cases, platform differences) are at `.wayfinder/proxy-examples.md`.
- Estimated effort: 2–3 days implementation + 1 day testing.
