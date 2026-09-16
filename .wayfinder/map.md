---
type: wayfinder-map
status: complete
created: 2026-09-15
completed: 2026-09-15
---

# Proxy Support for Non-Kube Sites

## Destination

✅ **REACHED** - Design specification document created at `.wayfinder/proxy-support-design-spec.md`

A design specification document that describes how to extend HTTP proxy support for Links from Kubernetes sites to local system sites (podman, docker, systemd). The spec should be detailed enough to hand off to an implementer, covering storage format, file paths, rendering logic, and user workflow - mirroring the existing K8s implementation pattern.

## Notes

**Domain**: Skupper site networking, inter-site Links, HTTP proxy configuration

**Reference Implementation**: Study `/internal/kube/site/site.go`, `/internal/kube/secrets/sync.go`, and `/internal/kube/adaptor/config_sync.go` for K8s proxy handling patterns.

**Key Constraint**: Must maintain API consistency with K8s - the Link YAML `settings: {proxy-configuration: <name>}` reference pattern stays identical. Only the backing storage changes from K8s Secret API to file-based Secret YAML.

**Skills to Consult**: 
- `mattpocock-skills:domain-modeling` - for maintaining consistency with existing Site/Link/Secret model
- `mattpocock-skills:grilling` - when design decisions surface edge cases

**Target Platforms**: podman, docker, linux/systemd (collectively "local system sites")

## Decisions so far

- [K8s Proxy Implementation Analysis](tickets/001-k8s-proxy-implementation-analysis.md) — Complete reference document at `.wayfinder/k8s-proxy-implementation.md` mapping K8s flow from Secret to router config
- [Secret YAML Schema Design](tickets/002-secret-yaml-schema-design.md) — Proxy Secrets use native K8s format with `type: kubernetes.io/basic-auth`, stored at `input/resources/Secret-<name>.yaml` with stringData fields (host, port, username, password)
- [File System Layout Design](tickets/003-file-system-layout-design.md) — Password files at `runtime/proxies/<name>/password.txt`, containers mount to `/etc/skupper-router/runtime/proxies/`, systemd uses absolute paths, permissions 0600 for passwords
- [Router Config Rendering Design](tickets/004-router-config-rendering-design.md) — Add createProxyProfiles() function, getProxyConfig() method in SiteState, write passwords after router config generation, log warnings for missing Secrets
- [Container Volume Mount Design](tickets/005-container-volume-mount-design.md) — Add FileMount struct to compat and bundle renderers, mount runtime/proxies to /etc/skupper-router/runtime/proxies with SELinux "z" option
- [Example Configurations](tickets/006-example-configurations.md) — Complete examples at `.wayfinder/proxy-examples.md` covering authenticated/unauthenticated proxies, multi-namespace, error cases, platform differences, and user workflows

## Not yet specified

*(All design questions resolved - ready to hand off to implementer)*

## Out of scope

- **Migration from custom proxy workarounds**: No migration needed - this is a new feature, not replacing existing functionality
- **Proxy for RouterAccess (inbound links)**: Only outbound Links need proxy configuration. Inbound RouterAccess connections don't traverse proxies.
- **Runtime credential rotation**: Dynamic updates when proxy Secrets change is a future enhancement. MVP requires site reload to pick up Secret changes.
- **CLI helper for Secret creation**: No `skupper secret create` command for MVP. Users manually create Secret YAML files.
- **Validation beyond basic schema checks**: No deep validation of proxy connectivity or credentials at config time. Invalid proxy config fails at runtime when Link attempts connection.
