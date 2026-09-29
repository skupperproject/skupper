# 02: Container proxy mounts

**What to build:** Podman/docker sites and bundle-deployed sites mount the `runtime/proxies/` directory into the router container at `/etc/skupper-router/runtime/proxies/`, so the containerized router can read proxy password files written by the rendering pipeline. The mount follows the same pattern as the existing certs mount: bind mount with SELinux `z` option. The bundle renderer uses template variables (`{{.NamespacesPath}}`, `{{.Namespace}}`) in the source path, matching the certs mount pattern.

After this, a site operator can create a proxy Secret, reference it from a Link, run `skupper system reload`, and the router container connects through the proxy — the full end-to-end flow works on podman and docker platforms.

**Blocked by:** 01: Core proxy rendering and tests.

**Status:** ready-for-agent

- [ ] Compat renderer's container preparation includes a FileMount from `runtime/proxies` to `/etc/skupper-router/runtime/proxies` with SELinux `z` option, alongside the existing config and certs mounts
- [ ] Bundle renderer's container preparation includes the same FileMount using template variables for the source path
- [ ] A podman/docker site with a proxied Link can be rendered and the router container's mount list includes the proxies directory (verifiable via `podman inspect`)
