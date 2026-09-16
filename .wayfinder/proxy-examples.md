# Proxy Configuration Examples for Non-Kube Sites

Complete examples showing proxy configuration workflow from user's perspective.

## Example 1: Authenticated Proxy

### Scenario
Two sites need to link through a corporate HTTP proxy at `proxy.example.com:3128` requiring basic authentication.

### Directory Structure (Before Rendering)

```
~/.local/share/skupper/namespaces/default/
├── input/
│   └── resources/
│       ├── Site-my-site.yaml
│       ├── Secret-my-proxy-config.yaml          # ← Proxy Secret
│       ├── Link-link-to-remote.yaml             # ← References proxy
│       └── RouterAccess-skupper-local.yaml
└── runtime/                                      # ← Empty before rendering
```

### Step 1: Create Proxy Secret YAML

**File**: `~/.local/share/skupper/namespaces/default/input/resources/Secret-my-proxy-config.yaml`

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-proxy-config
  namespace: default
type: kubernetes.io/basic-auth
stringData:
  host: proxy.example.com
  port: "3128"
  username: myuser
  password: <your-proxy-password>
```

### Step 2: Create Link YAML Referencing Proxy

**File**: `~/.local/share/skupper/namespaces/default/input/resources/Link-link-to-remote.yaml`

```yaml
apiVersion: skupper.io/v2alpha1
kind: Link
metadata:
  name: link-to-remote
  namespace: default
spec:
  cost: 1
  endpoints:
  - host: remote-site.example.com
    name: inter-router
    port: "55671"
  - host: remote-site.example.com
    name: edge
    port: "45671"
  tlsCredentials: link-to-remote
  settings:
    proxy-configuration: my-proxy-config    # ← References proxy Secret
```

### Step 3: Render Configuration

```bash
# For podman/docker:
skupper system reload

# For systemd:
skupper system reload
```

### Directory Structure (After Rendering)

```
~/.local/share/skupper/namespaces/default/
├── input/
│   └── resources/
│       ├── Secret-my-proxy-config.yaml
│       └── Link-link-to-remote.yaml
└── runtime/
    ├── router/
    │   └── skrouterd.json                       # ← Router config with ProxyProfile
    ├── certs/
    │   └── link-to-remote/
    │       ├── ca.crt
    │       ├── tls.crt
    │       └── tls.key
    └── proxies/                                 # ← NEW
        └── my-proxy-config/
            └── password.txt                     # ← Contains proxy password (0600)
```

### Rendered Router Config JSON Snippet

**File**: `~/.local/share/skupper/namespaces/default/runtime/router/skrouterd.json`

```json
{
  "proxyProfiles": [
    {
      "name": "my-proxy-config",
      "host": "proxy.example.com",
      "port": "3128",
      "username": "myuser",
      "password": "file:/etc/skupper-router/runtime/proxies/my-proxy-config/password.txt"
    }
  ],
  "connectors": [
    {
      "name": "link-to-remote",
      "host": "remote-site.example.com",
      "port": "55671",
      "role": "inter-router",
      "cost": 1,
      "sslProfile": "link-to-remote-profile",
      "proxyProfile": "my-proxy-config"
    }
  ]
}
```

### Container Mount (Podman/Docker)

The router container sees:

```
/etc/skupper-router/
├── config/
│   └── skrouterd.json
├── runtime/
    ├── certs/
    │   └── link-to-remote/
    └── proxies/                                 # ← Mounted from host
        └── my-proxy-config/
            └── password.txt
```

**Mount configuration** (in container):
- Host: `~/.local/share/skupper/namespaces/default/runtime/proxies`
- Container: `/etc/skupper-router/runtime/proxies`

---

## Example 2: Unauthenticated Proxy

### Scenario
Link through an internal proxy at `proxy.internal.corp:8080` that doesn't require authentication.

### Proxy Secret YAML

**File**: `~/.local/share/skupper/namespaces/default/input/resources/Secret-internal-proxy.yaml`

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: internal-proxy
  namespace: default
type: kubernetes.io/basic-auth
stringData:
  host: proxy.internal.corp
  port: "8080"
  # username and password omitted for unauthenticated proxy
```

### Link YAML

```yaml
apiVersion: skupper.io/v2alpha1
kind: Link
metadata:
  name: link-via-internal-proxy
spec:
  endpoints:
  - host: remote.example.com
    name: inter-router
    port: "55671"
  tlsCredentials: link-via-internal-proxy
  settings:
    proxy-configuration: internal-proxy
```

### After Rendering

**Directory structure**:
```
runtime/
└── proxies/
    └── internal-proxy/       # ← Directory created but empty (no password.txt)
```

**Router config JSON**:
```json
{
  "proxyProfiles": [
    {
      "name": "internal-proxy",
      "host": "proxy.internal.corp",
      "port": "8080"
      // No username or password fields
    }
  ],
  "connectors": [
    {
      "name": "link-via-internal-proxy",
      "proxyProfile": "internal-proxy"
    }
  ]
}
```

**Note**: No `password.txt` file created since credentials are absent.

---

## Example 3: Multi-Namespace Scenario

### Scenario
Two namespaces (`prod` and `dev`), each with different proxy configurations.

### Production Namespace

**Directory**: `~/.local/share/skupper/namespaces/prod/`

```
input/resources/
├── Secret-prod-proxy.yaml         # host: proxy-prod.corp, port: 3128
└── Link-prod-link.yaml            # settings: {proxy-configuration: prod-proxy}

runtime/proxies/
└── prod-proxy/
    └── password.txt
```

### Development Namespace

**Directory**: `~/.local/share/skupper/namespaces/dev/`

```
input/resources/
├── Secret-dev-proxy.yaml          # host: proxy-dev.corp, port: 8080
└── Link-dev-link.yaml             # settings: {proxy-configuration: dev-proxy}

runtime/proxies/
└── dev-proxy/
    └── password.txt
```

**Isolation**: Each namespace has completely separate `runtime/proxies/` directories. No cross-namespace access.

---

## Example 4: Error Case - Missing Proxy Secret

### Link YAML References Non-Existent Secret

```yaml
apiVersion: skupper.io/v2alpha1
kind: Link
metadata:
  name: broken-link
spec:
  endpoints:
  - host: remote.example.com
    port: "55671"
  tlsCredentials: broken-link
  settings:
    proxy-configuration: missing-proxy    # ← Secret doesn't exist
```

### Rendering Behavior

**Console output**:
```
Warning: proxy Secret "missing-proxy" not found for Link "broken-link"
```

**Result**:
- Link is created WITHOUT proxy configuration (ProxyConfig = nil)
- No proxyProfile in router connector JSON
- Link may still work if direct network access is available
- Site rendering completes successfully (non-fatal error)

---

## Example 5: Platform Differences

### Podman Site

**Router sees**:
- Config: `/etc/skupper-router/config/skrouterd.json`
- Password: `/etc/skupper-router/runtime/proxies/my-proxy/password.txt`

**ProxyProfile in JSON**:
```json
{
  "password": "file:/etc/skupper-router/runtime/proxies/my-proxy/password.txt"
}
```

### Linux/Systemd Site

**Router sees** (absolute filesystem paths):
- Config: `/home/user/.local/share/skupper/namespaces/default/runtime/router/skrouterd.json`
- Password: `/home/user/.local/share/skupper/namespaces/default/runtime/proxies/my-proxy/password.txt`

**ProxyProfile in JSON**:
```json
{
  "password": "file:/home/user/.local/share/skupper/namespaces/default/runtime/proxies/my-proxy/password.txt"
}
```

**Note**: Systemd uses absolute paths because `SslProfileBasePath` (and `ProxyProfileBasePath`) is set to the actual site home directory, not `/etc/skupper-router`.

---

## Verification Commands

### Check Rendered Password File

```bash
# List proxy profiles
ls -la ~/.local/share/skupper/namespaces/default/runtime/proxies/

# Check password file permissions
ls -la ~/.local/share/skupper/namespaces/default/runtime/proxies/my-proxy-config/password.txt
# Should show: -rw------- (0600)

# View password content
cat ~/.local/share/skupper/namespaces/default/runtime/proxies/my-proxy-config/password.txt
# Shows: <your-proxy-password>
```

### Check Router Config

```bash
# View router config
cat ~/.local/share/skupper/namespaces/default/runtime/router/skrouterd.json | jq '.proxyProfiles'

# Expected output:
# [
#   {
#     "name": "my-proxy-config",
#     "host": "proxy.example.com",
#     "port": "3128",
#     "username": "myuser",
#     "password": "file:/etc/skupper-router/runtime/proxies/my-proxy-config/password.txt"
#   }
# ]
```

### Verify Container Mount (Podman)

```bash
# Inspect container mounts
podman inspect default-skupper-router | jq '.[0].Mounts[] | select(.Destination == "/etc/skupper-router/runtime/proxies")'

# Expected output:
# {
#   "Type": "bind",
#   "Source": "/home/user/.local/share/skupper/namespaces/default/runtime/proxies",
#   "Destination": "/etc/skupper-router/runtime/proxies",
#   "Options": ["z"]
# }

# Check inside container
podman exec default-skupper-router ls -la /etc/skupper-router/runtime/proxies/my-proxy-config/
# Should show password.txt
```

---

## User Workflow Summary

### For Authenticated Proxy

1. **Create proxy Secret YAML** with host, port, username, password
2. **Create/edit Link YAML** to add `settings: {proxy-configuration: <secret-name>}`
3. **Render**: `skupper system reload`
4. **Verify**: Check that password.txt exists with 0600 permissions

### For Unauthenticated Proxy

1. **Create proxy Secret YAML** with only host and port (omit username/password)
2. **Create/edit Link YAML** to reference the Secret
3. **Render**: `skupper system reload`
4. **Verify**: No password.txt created, but proxyProfile in router config

### Troubleshooting

**Link not working through proxy?**
- Check Secret name matches Link's `proxy-configuration` setting
- Verify host/port are correct in Secret
- Check password.txt exists (for authenticated) with correct permissions
- Verify proxy allows CONNECT to ports 55671 and 45671
- Check router logs: `journalctl -u skupper-default.service` (systemd) or `podman logs default-skupper-router`
