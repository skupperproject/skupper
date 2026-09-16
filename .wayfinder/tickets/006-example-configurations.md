---
id: 006
title: Example Configurations
type: prototype
status: closed
blocks: [002]
created: 2026-09-15
resolved: 2026-09-15
---

## Question

What do complete, working proxy configurations look like from the user's perspective? Create concrete examples showing the full workflow.

**Deliverables**:

1. **Proxy Secret YAML example**: A complete, valid proxy Secret file (both with and without authentication)
2. **Link YAML example**: A Link referencing the proxy Secret via `settings`
3. **Directory structure diagram**: Show where files live on disk before and after rendering
4. **End-to-end workflow**: Step-by-step user commands/file edits to set up a proxied link
5. **Rendered output samples**: What the router config JSON looks like, what `password.txt` contains

**Use cases to cover**:
- Authenticated proxy (username/password)
- Unauthenticated proxy (no credentials)
- Multi-namespace scenario (two namespaces, different proxies)

**Constraints**:
- Must use schema from ticket #002
- File paths must match ticket #003
- Should mirror K8s user experience from https://skupper.io/docs/kube-cli/site-linking.html#linking-sites-through-an-http-proxy

**Dependencies**: Blocked by #002 (Secret schema) - can't write examples until schema is defined.

**Why**: These examples become user documentation and test fixtures. They surface usability issues before implementation.

**How to apply**: Use as smoke tests during implementation, copy directly into user docs.

---

## Resolution

**Deliverable created**: `/home/paulwright/repos/sk/skupper/.wayfinder/proxy-examples.md`

Complete examples document covering:

### 1. Authenticated Proxy (Example 1)
- Complete directory structure before/after rendering
- Proxy Secret YAML with credentials
- Link YAML referencing proxy
- Rendered router config JSON snippet
- Container mount visualization

### 2. Unauthenticated Proxy (Example 2)
- Secret YAML without username/password
- Behavior: no password.txt file created
- Router config JSON without credentials

### 3. Multi-Namespace Scenario (Example 3)
- Two namespaces with different proxies
- Shows namespace isolation
- Separate runtime/proxies directories per namespace

### 4. Error Case - Missing Proxy Secret (Example 4)
- Link references non-existent Secret
- Warning logged, Link created without proxy
- Non-fatal error (rendering continues)

### 5. Platform Differences (Example 5)
- Podman: router sees `/etc/skupper-router/runtime/proxies/...`
- Systemd: router sees absolute paths `~/.local/share/skupper/...`
- ProxyProfile password path differs by platform

### Verification Commands
- Check rendered password files
- Inspect router config JSON
- Verify container mounts (podman inspect)
- Inside-container verification

### User Workflow Summary
- Step-by-step for authenticated proxy
- Step-by-step for unauthenticated proxy
- Troubleshooting guide

All examples use schemas from ticket #002 and paths from ticket #003.
