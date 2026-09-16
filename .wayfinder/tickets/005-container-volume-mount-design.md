---
id: 005
title: Container Volume Mount Design
type: grilling
status: closed
blocks: [003]
created: 2026-09-15
resolved: 2026-09-15
---

## Question

How should podman/docker container configurations be updated to mount proxy profile directories? Design the volume mount changes needed for the router container to access rendered proxy configs.

**Areas to cover**:

1. **Mount point**: What host path gets mounted to `/etc/skupper-router-proxies/` in the container?
2. **Existing mounts**: How does this interact with existing cert mounts? Separate volume or extend existing?
3. **Bootstrap/install changes**: Where to update container creation (in `bootstrap/install.go` or container runtime files)?
4. **Read-only vs read-write**: Should proxy-profiles volume be read-only in container?
5. **Systemd case**: For linux/systemd, is any mount config needed, or does the router process just read from filesystem?
6. **Testing**: How to verify mounts work without running full site?

**Constraints**:
- Follow existing mount patterns (see how `/etc/skupper-router-certs` is mounted)
- Must work for both podman and docker runtimes
- Path inside container must match ticket #003's design

**Dependencies**: Blocked by #003 (file system layout) - need to know what host path to mount.

**Why**: Wrong mounts mean router can't see password files even if they're rendered correctly.

**How to apply**: Exact code changes needed in bootstrap and container client code.

---

## Resolution

### 1. Compat Renderer (Podman/Docker)

**File**: `internal/nonkube/compat/site_state_renderer.go`

**Add to FileMounts array** at line 244 (after certs mount):

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
    // NEW: Proxy profiles mount
    {
        Source:      path.Join(siteConfigPath, string(api.ProxyProfilesPath)),
        Destination: "/etc/skupper-router/runtime/proxies",
        Options:     []string{"z"},
    },
},
```

### 2. Bundle Renderer

**File**: `internal/nonkube/bundle/site_state_renderer.go`

**Add to FileMounts array** at line 126 (after certs mount):

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
    // NEW: Proxy profiles mount
    {
        Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", string(api.ProxyProfilesPath)),
        Destination: "/etc/skupper-router/runtime/proxies",
        Options:     []string{"z"},
    },
},
```

### 3. Mount Configuration Details

**Source path**:
- Compat: `path.Join(siteConfigPath, string(api.ProxyProfilesPath))`
  - Expands to: `~/.local/share/skupper/namespaces/<ns>/runtime/proxies`
- Bundle: `path.Join("{{.NamespacesPath}}", "{{.Namespace}}", string(api.ProxyProfilesPath))`
  - Templates substituted at bundle extraction time

**Destination**: `/etc/skupper-router/runtime/proxies` (identical for both)

**Options**: `[]string{"z"}` - SELinux relabeling for container access

**Read/Write**: Read-write (no `ro` option) - matches certs mount pattern

### 4. Systemd (Linux Platform)

**No changes needed**. Systemd runs the router as a native process with no container, so it reads directly from the filesystem. The router sees absolute paths like:
- `~/.local/share/skupper/namespaces/<ns>/runtime/proxies/<name>/password.txt`

### 5. Testing the Mount

After implementation, verify mount with:

```bash
# For podman
podman inspect <namespace>-skupper-router | grep -A 10 "Mounts"

# Should show:
# Source: /home/user/.local/share/skupper/namespaces/<ns>/runtime/proxies
# Destination: /etc/skupper-router/runtime/proxies

# Verify inside container
podman exec <namespace>-skupper-router ls -la /etc/skupper-router/runtime/proxies/
```

### Summary

**Two files to modify**, each adds 5 lines (one FileMount struct):
1. `internal/nonkube/compat/site_state_renderer.go:244`
2. `internal/nonkube/bundle/site_state_renderer.go:126`

**Pattern**: Exact mirror of certs mount, just different path constants.
