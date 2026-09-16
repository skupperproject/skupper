---
id: 001
title: K8s Proxy Implementation Analysis
type: research
status: closed
blocks: []
created: 2026-09-15
resolved: 2026-09-15
---

## Question

What is the complete flow of HTTP proxy configuration in the K8s implementation, from user-created Secret through to router config JSON? Document:

1. Secret structure (fields, type, where it lives)
2. How ProfilesWatcher detects and syncs proxy Secrets
3. What gets written to `/etc/skupper-router-proxies/` and when
4. How the router config JSON ProxyProfile is populated
5. How Link YAML references the proxy Secret via `settings["proxy-configuration"]`
6. Error cases: missing Secret, malformed Secret, Secret updates

**Deliverable**: A reference document mapping the K8s code flow with file paths, function names, and data transformations at each step. This becomes the template for the non-kube design.

**Why**: Need complete picture of K8s implementation to ensure non-kube design maintains parity.

**How to apply**: Use this document as the authoritative reference when designing non-kube components - every K8s step should have a non-kube equivalent.

---

## Resolution

**Deliverable created**: `/home/paulwright/repos/sk/skupper/.wayfinder/k8s-proxy-implementation.md`

Complete reference document covering:
- Secret structure and YAML examples
- ProfilesWatcher detection mechanism
- Disk synchronization (password file writing)
- Router config population (ProxyProfile JSON)
- Link reference via settings["proxy-configuration"]
- Error cases and handling
- Complete example flow with YAML/JSON
- Non-kube translation guide
- File reference map with line numbers

**Key findings for non-kube design**:
1. Only password written to disk (`/etc/skupper-router-proxies/{name}/password.txt`)
2. Host/port/username go directly in ProxyProfile JSON
3. Password referenced as `file:/etc/skupper-router-proxies/{name}/password.txt`
4. Cache-first retrieval pattern for performance
5. Type detection via `kubernetes.io/basic-auth`
