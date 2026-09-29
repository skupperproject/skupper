# 01: Core proxy rendering and tests

**What to build:** A Link on a non-kube site with `settings.proxy-configuration` pointing to a `kubernetes.io/basic-auth` Secret produces a ProxyProfile entry in the router config JSON and a `password.txt` file at `runtime/proxies/<name>/password.txt` with 0600 permissions. The proxy Secret is a standard Kubernetes Secret YAML stored in `input/resources/`, loaded via the existing Secret loading mechanism — no new loaders or discovery needed.

For unauthenticated proxies (Secret with only `host` and `port`, no `username`/`password`), a ProxyProfile with only host and port appears in the router config, and no password file is written.

When a Link references a proxy Secret that doesn't exist or has the wrong type or missing required fields, a warning is logged to stderr and the Link is created without proxy configuration. Rendering continues (non-fatal). Filesystem errors (can't create directory or write file) halt rendering (fatal).

The proxy path is derived from the existing SslProfileBasePath — no platform-specific renderer changes needed. On linux/systemd this means the router reads password files at absolute filesystem paths. On containers, the path resolves through the container mount (added in ticket 02).

Tests extend the existing `Render()` seam in the fs_config_renderer test, using the established `fakeSiteState()` pattern with proxy-configured fixtures.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] SiteState has a method that extracts proxy configuration from a `kubernetes.io/basic-auth` Secret referenced by a Link's `settings.proxy-configuration`, returning host, port, username, and a profile path built from the SSL base path + `runtime/proxies`
- [ ] `linkMap()` passes the extracted proxy config to `site.NewLink()` instead of an empty ProxyConfig (replacing the existing TODO)
- [ ] FileSystemConfigurationRenderer has a `createProxyProfiles()` function that iterates Links, finds their proxy Secrets, and writes `password.txt` (content = password bytes, permissions = 0600) into `runtime/proxies/<secret-name>/`
- [ ] `createProxyProfiles()` is called from `Render()` after `createRouterConfig()` and before `createTlsCertificates()`
- [ ] Authenticated proxy test: Link with proxy-configuration setting + Secret with credentials → after Render(), `runtime/proxies/<name>/password.txt` exists with correct content and 0600 permissions, and `skrouterd.json` contains a ProxyProfile with host, port, username, and `file:` password path, and the connector references the ProxyProfile by name
- [ ] Unauthenticated proxy test: Secret with only host/port → after Render(), no password file exists, and ProxyProfile in router config has host and port but no username or password
- [ ] Missing Secret test: Link references non-existent Secret → after Render(), no ProxyProfile in router config, render succeeds without error
- [ ] Regression guard test: Link without proxy settings → after Render(), no proxy artifacts, no ProxyProfile (existing behavior preserved)
