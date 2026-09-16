# Kubernetes HTTP Proxy Implementation Reference

Complete documentation of HTTP proxy configuration flow in Skupper's Kubernetes implementation.

## Overview

The K8s implementation uses Kubernetes Secrets (type `kubernetes.io/basic-auth`) to store proxy credentials, syncs them to disk, and references them in router configuration through Link resources.

## Flow Diagram

```
User Creates Secret
         │
         ▼
   ProfilesWatcher (manager.go)
    detects basic-auth secret
         │
         ▼
   Sync.handleProxyProfile (sync.go)
    processes secret data
         │
         ▼
   writeProxyProfile (sync.go)
    writes password to disk
    /etc/skupper-router-proxies/{name}/password.txt
         │
         ▼
   ExpectProxyProfiles (sync.go)
    validates and prepares profiles
         │
         ▼
   ConfigSync.syncProxyProfileCredentialsToDisk
    coordinates secret → disk sync
         │
         ▼
   User Creates Link
    references proxy via settings["proxy-configuration"]
         │
         ▼
   Site.getProxyConfig (site.go)
    retrieves secret data from cache
         │
         ▼
   Link.Apply (link.go)
    creates ProxyProfile
         │
         ▼
   ConfigureProxyProfile (qdr.go)
    builds ProxyProfile JSON
         │
         ▼
   RouterConfig.AddProxyProfile
    adds to router config
         │
         ▼
   SyncProxyProfilesToRouter (sync_router_ops.go)
    pushes to router via AMQP agent
```

## 1. Secret Structure

### Secret Type
- **Type**: `kubernetes.io/basic-auth`
- **Namespace**: Same as Skupper site

### Required Fields
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-proxy
  namespace: skupper
type: kubernetes.io/basic-auth
data:
  host: <base64-encoded-proxy-host>      # e.g., "proxy.example.com"
  port: <base64-encoded-proxy-port>      # e.g., "8080"
  username: <base64-encoded-username>    # e.g., "proxyuser"
  password: <base64-encoded-password>    # e.g., "proxypass"
```

### Field Details
- **host**: Proxy server hostname or IP address
- **port**: Proxy server port number (as string)
- **username**: Proxy authentication username (optional if no auth)
- **password**: Proxy authentication password (optional if no auth)

### Example (decoded values)
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: corporate-proxy
  namespace: skupper
type: kubernetes.io/basic-auth
stringData:
  host: proxy.corp.example.com
  port: "3128"
  username: alice
  password: <your-password>
```

## 2. ProfilesWatcher Detection

**File**: `/internal/kube/secrets/manager.go`

### Detection Logic (lines 122-140)

```go
case secret.Type == "kubernetes.io/basic-auth":
    state, ok := w.state[secretName]
    if ok {
        if state.SecretKey == "" {
            state.SecretKey = key
            updateSecretChecksum(secret, &state.SecretContentSum)
        } else if state.SecretKey == key {
            if updateSecretChecksum(secret, &state.SecretContentSum) {
                changed = true
            }
        }
    }
    if !changed {
        return nil
    }
    w.logger.Info("ProxyProfile Secret Changed",
        slog.String("name", secretName),
    )
    return w.update(w)
```

### Behavior
- **Trigger**: Any Secret with `type: kubernetes.io/basic-auth`
- **Tracking**: Maintains state with secret key and content checksum
- **Change Detection**: Compares SHA-256 checksum of secret data
- **Updates**: Calls `w.update(w)` to trigger router config update

### State Management
```go
type profileWatcherContext struct {
    Ordinal            uint64
    OldestValidOrdinal uint64
    SecretKey          string        // "namespace/secretname"
    SecretContentSum   [32]byte      // SHA-256 checksum
}
```

## 3. Disk Synchronization

**File**: `/internal/kube/secrets/sync.go`

### handleProxyProfile (lines 123-151)

```go
func (s *Sync) handleProxyProfile(namespace string, secret *corev1.Secret) (bool, error) {
    profileName := secret.Name
    prev, hadPrev := s.getProfile(profileName)
    proxyProfile, isConfigured := s.getConfiguredProxy(profileName)
    updated := syncContext{
        profileContext: profileContext{
            ProfileName: profileName,
        },
        SecretKey: namespace + "/" + profileName,
    }
    if !isConfigured {
        s.setProfileSecret(updated)
        return false, nil
    }
    sumChanged := updateSecretChecksum(secret, &prev.SecretContentSum)
    updated.SecretContentSum = prev.SecretContentSum
    hasWrite := false
    if !hadPrev || sumChanged {
        if len(secret.Data["username"]) > 0 && len(secret.Data["password"]) > 0 {
            path := strings.TrimPrefix(proxyProfile.Password, "file:")
            if err := writeProxyProfile(secret, path); err != nil {
                return false, fmt.Errorf("write for proxyProfile failed: %s", err)
            }
            hasWrite = true
        }
    }
    s.setProfileSecret(updated)
    return hasWrite, nil
}
```

### Logic Flow
1. **Profile Lookup**: Gets previous state and configured profile
2. **Configuration Check**: Only writes if profile is configured in router config
3. **Checksum Validation**: Compares checksum to detect changes
4. **Credential Check**: Requires both username AND password to be non-empty
5. **Disk Write**: Writes password.txt if credentials changed

### writeProxyProfile (lines 348-361)

```go
func writeProxyProfile(secret *corev1.Secret, filePath string) error {
    _, ok := secret.Data["password"]
    if !ok {
        return fmt.Errorf("empty proxyProfile %q", secret.Name)
    }
    baseName := path.Dir(filePath)
    if err := os.MkdirAll(baseName, 0755); err != nil {
        return fmt.Errorf("error making proxyProfile password directory %q: %e", baseName, err)
    }
    if err := writeFile(filePath, []byte(secret.Data["password"]), 0644); err != nil {
        return fmt.Errorf("error writing password.txt: %e", err)
    }
    return nil
}
```

### Disk Layout
```
/etc/skupper-router-proxies/
├── proxy-name-1/
│   └── password.txt          # Contains password from secret
├── proxy-name-2/
│   └── password.txt
└── ...
```

### File Permissions
- **Directory**: 0755 (rwxr-xr-x)
- **password.txt**: 0644 (rw-r--r--)

### Data Transformation
```
Secret.Data["password"] (base64 in K8s)
        ↓
Decoded bytes
        ↓
/etc/skupper-router-proxies/{secret-name}/password.txt (raw bytes)
```

## 4. Router Config Population

**File**: `/internal/qdr/qdr.go`

### ProxyProfile Struct (lines 588-594)

```go
type ProxyProfile struct {
    Name     string `json:"name,omitempty"`
    Host     string `json:"host,omitempty"`
    Port     string `json:"port,omitempty"`
    Username string `json:"username,omitempty"`
    Password string `json:"password,omitempty"`
}
```

### ConfigureProxyProfile (lines 220-230)

```go
const PROXY_PATH_PREFIX = "file:"
const PROXY_PASSWORD_FILE = "password.txt"

func ConfigureProxyProfile(name string, host string, port string, username string, path string) ProxyProfile {
    profile := ProxyProfile{
        Name: name,
        Host: host,
        Port: port,
    }
    if username != "" && path != "" {
        profile.Username = username
        profile.Password = path_.Join(PROXY_PATH_PREFIX, path, name, PROXY_PASSWORD_FILE)
    }
    return profile
}
```

### Data Mapping

| Source | Destination Field | Notes |
|--------|------------------|-------|
| Secret name | ProxyProfile.Name | Direct mapping |
| Secret.Data["host"] | ProxyProfile.Host | From secret, not disk |
| Secret.Data["port"] | ProxyProfile.Port | From secret, not disk |
| Secret.Data["username"] | ProxyProfile.Username | From secret, not disk |
| Disk file path | ProxyProfile.Password | `file:/etc/skupper-router-proxies/{name}/password.txt` |

### Example ProxyProfile JSON
```json
{
  "name": "corporate-proxy",
  "host": "proxy.corp.example.com",
  "port": "3128",
  "username": "alice",
  "password": "file:/etc/skupper-router-proxies/corporate-proxy/password.txt"
}
```

### Why Password on Disk?
- **Security**: Avoids password in router config JSON (which may be logged/exposed)
- **File Reference**: Router reads password from file at runtime
- **Prefix**: `file:` tells router to read from filesystem

## 5. Link Reference

**File**: `/pkg/apis/skupper/v2alpha1/types.go`

### LinkSpec Structure (lines 584-596)

```go
type LinkSpec struct {
    Endpoints      []Endpoint        `json:"endpoints"`
    TlsCredentials string            `json:"tlsCredentials,omitempty"`
    Cost           int               `json:"cost,omitempty"`
    Settings       map[string]string `json:"settings,omitempty"`
}

func (s *LinkSpec) GetProxyConfiguration() string {
    if value, ok := s.Settings["proxy-configuration"]; ok {
        return value
    }
    return ""
}
```

### Link YAML Example
```yaml
apiVersion: skupper.io/v2alpha1
kind: Link
metadata:
  name: site-to-site
  namespace: skupper
spec:
  endpoints:
    - name: inter-router
      host: remote-site.example.com
      port: "55671"
  tlsCredentials: link-tls-credentials
  cost: 1
  settings:
    proxy-configuration: corporate-proxy  # References Secret name
```

### Reference Flow

**File**: `/internal/kube/site/site.go` (lines 1312-1326)

```go
func (s *Site) getProxyConfig(proxySetting string) (*site.ProxyConfig, error) {
    if proxySetting != "" {
        proxySecret, err := s.profiles.Cache.Get(s.namespace + "/" + proxySetting)
        if proxySecret != nil && err == nil {
            return &site.ProxyConfig{
                Host:        string(proxySecret.Data["host"]),
                Port:        string(proxySecret.Data["port"]),
                User:        string(proxySecret.Data["username"]),
                ProfilePath: PROXY_PROFILE_PATH}, nil
        } else {
            return nil, stderrors.New("Secret not found for proxy configuration")
        }
    }
    return nil, nil
}
```

**Constants**:
```go
const PROXY_PROFILE_PATH = "/etc/skupper-router-proxies"
```

### ProxyConfig to ProxyProfile

**File**: `/internal/site/link.go` (lines 40-78)

```go
func (l *Link) Apply(current *qdr.RouterConfig) bool {
    // ... endpoint and SSL profile setup ...
    
    proxyProfileName := proxyProfileName(l.definition)
    prevProxyProfileName := current.Connectors[l.name].ProxyProfile
    
    connector := qdr.Connector{
        Name:         l.name,
        Cost:         cost,
        SslProfile:   sslProfileName,
        ProxyProfile: proxyProfileName,  // References ProxyProfile name
        Role:         role,
        Host:         endpoint.Host,
        Port:         endpoint.Port,
    }
    current.AddConnector(connector)
    
    if proxyProfileName != "" {
        current.AddProxyProfile(qdr.ConfigureProxyProfile(
            proxyProfileName,
            l.proxyConfig.Host,
            l.proxyConfig.Port,
            l.proxyConfig.User,
            l.proxyConfig.ProfilePath))
        if prevProxyProfileName != "" && prevProxyProfileName != proxyProfileName {
            current.RemoveProxyProfile(prevProxyProfileName)
        }
    } else if prevProxyProfileName != "" {
        current.RemoveProxyProfile(prevProxyProfileName)
    }
    return true
}

func proxyProfileName(link *skupperv2alpha1.Link) string {
    return link.Spec.GetProxyConfiguration()
}
```

### Data Flow Summary
```
Link.Spec.Settings["proxy-configuration"] = "corporate-proxy"
        ↓
getProxyConfig("corporate-proxy")
        ↓
Retrieves Secret from cache
        ↓
Creates site.ProxyConfig {
    Host: "proxy.corp.example.com",
    Port: "3128",
    User: "alice",
    ProfilePath: "/etc/skupper-router-proxies"
}
        ↓
Link.Apply calls ConfigureProxyProfile
        ↓
Creates qdr.ProxyProfile {
    Name: "corporate-proxy",
    Host: "proxy.corp.example.com",
    Port: "3128",
    Username: "alice",
    Password: "file:/etc/skupper-router-proxies/corporate-proxy/password.txt"
}
        ↓
Added to RouterConfig.ProxyProfiles
        ↓
Referenced by Connector.ProxyProfile = "corporate-proxy"
```

## 6. Router Synchronization

**File**: `/internal/qdr/sync_router_ops.go` (lines 130-164)

```go
func SyncProxyProfilesToRouter(agentPool *AgentPool, desired map[string]ProxyProfile) error {
    agent, err := agentPool.Get()
    if err != nil {
        return err
    }
    defer agentPool.Put(agent)

    actual, err := agent.GetProxyProfiles()
    if err != nil {
        return err
    }

    // Create or update profiles
    for _, profile := range desired {
        current, ok := actual[profile.Name]
        if !ok {
            if err := agent.CreateProxyProfile(profile); err != nil {
                return err
            }
            continue
        }
        if current != profile {
            if err := agent.UpdateProxyProfile(profile); err != nil {
                return err
            }
        }
    }
    
    // Delete removed profiles
    for _, profile := range actual {
        if _, ok := desired[profile.Name]; !ok {
            if err := agent.Delete("io.skupper.router.proxyProfile", profile.Name); err != nil {
                return err
            }
        }
    }
    return nil
}
```

### Sync Operations
1. **Get Current State**: Queries router for existing ProxyProfile entities
2. **Create**: Adds new profiles not present in router
3. **Update**: Updates profiles where fields have changed
4. **Delete**: Removes profiles no longer in desired state

### AMQP Agent Communication
- **Protocol**: AMQP management protocol
- **Entity Type**: `io.skupper.router.proxyProfile`
- **Operations**: CREATE, UPDATE, DELETE
- **Connection**: `amqp://localhost:5672`

### Connector JSON with Proxy
```json
{
  "name": "site-to-site",
  "role": "inter-router",
  "host": "remote-site.example.com",
  "port": "55671",
  "cost": 1,
  "sslProfile": "link-tls-credentials-profile",
  "proxyProfile": "corporate-proxy"
}
```

## 7. Error Cases

### Secret Not Found
**Location**: `/internal/kube/site/site.go:1322`

```go
return nil, stderrors.New("Secret not found for proxy configuration")
```

**Trigger**: Link references proxy-configuration but Secret doesn't exist  
**Result**: Link creation fails with error

### Missing Password Field
**Location**: `/internal/kube/secrets/sync.go:349-352`

```go
_, ok := secret.Data["password"]
if !ok {
    return fmt.Errorf("empty proxyProfile %q", secret.Name)
}
```

**Trigger**: Secret missing `password` field  
**Result**: Disk write fails, profile not created

### Empty Credentials
**Location**: `/internal/kube/secrets/sync.go:141`

```go
if len(secret.Data["username"]) > 0 && len(secret.Data["password"]) > 0 {
    // Write to disk
}
```

**Trigger**: Either username or password is empty  
**Result**: Password file not written (no-auth proxy not supported with basic-auth type)

### Disk Write Failure
**Location**: `/internal/kube/secrets/sync.go:354-358`

```go
if err := os.MkdirAll(baseName, 0755); err != nil {
    return fmt.Errorf("error making proxyProfile password directory %q: %e", baseName, err)
}
if err := writeFile(filePath, []byte(secret.Data["password"]), 0644); err != nil {
    return fmt.Errorf("error writing password.txt: %e", err)
}
```

**Trigger**: Filesystem permissions issue, disk full, etc.  
**Result**: Returns error, profile not created

### Secret Update/Deletion
**Location**: `/internal/kube/secrets/manager.go:127-129`

```go
if updateSecretChecksum(secret, &state.SecretContentSum) {
    changed = true
}
```

**Behavior**:
- **Update**: New checksum triggers rewrite of password.txt
- **Delete**: Secret deletion detected by cache, proxy profile removed from router

### ConfigMap Update Flow
**Location**: `/internal/kube/adaptor/config_sync.go:92-122`

```go
func (c *ConfigSync) configEvent(key string, configmap *corev1.ConfigMap) error {
    if configmap == nil {
        return nil
    }
    desired, err := kubeqdr.GetRouterConfigFromConfigMap(configmap)
    if err != nil {
        return err
    }
    if err := c.syncSslProfileCredentialsToDisk(desired.SslProfiles); err != nil {
        return err
    }
    if err := qdr.SyncSslProfilesToRouter(c.agentPool, desired.SslProfiles); err != nil {
        return err
    }
    if err := c.syncProxyProfileCredentialsToDisk(key, desired.ProxyProfiles); err != nil {
        return err
    }
    if err := qdr.SyncProxyProfilesToRouter(c.agentPool, desired.ProxyProfiles); err != nil {
        return err
    }
    // ... continue with bridges and router config ...
}
```

### Missing Secret During ExpectProxyProfiles
**Location**: `/internal/kube/secrets/sync.go:260-286`

```go
func (s *Sync) ExpectProxyProfiles(key string, profiles map[string]qdr.ProxyProfile) SyncDelta {
    var delta SyncDelta
    delta.ProxyUpdates = make(map[string]qdr.ProxyProfile)
    s.setConfiguredProxy(profiles)
    namespace, _, err := cache.SplitMetaNamespaceKey(key)
    if err != nil {
        delta.Errors = append(delta.Errors, err)
    }
    for profileName, profile := range profiles {
        secret, _ := s.cache.Get(namespace + "/" + profileName)
        if secret == nil {
            delta.Missing = append(delta.Missing, profileName)
            continue
        } else {
            _, err := s.handleProxyProfile(namespace, secret)
            if err != nil {
                delta.Errors = append(delta.Errors, err)
            }
            profile.Host = string(secret.Data["host"])
            profile.Port = string(secret.Data["port"])
            profile.Username = string(secret.Data["username"])
            delta.ProxyUpdates[profileName] = profile
        }
    }
    return delta
}
```

**Error Tracking**:
```go
type SyncDelta struct {
    Missing         []string                    // Secret not found
    PendingOrdinals map[string]OrdinalDelta    // For SSL profiles
    ProxyUpdates    map[string]qdr.ProxyProfile // Updated proxy data
    Errors          []error                     // Any errors during sync
}
```

## 8. Complete Example Flow

### Step 1: Create Proxy Secret
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-proxy
  namespace: west
type: kubernetes.io/basic-auth
stringData:
  host: proxy.example.com
  port: "8080"
  username: user1
  password: <your-password>
```

### Step 2: ProfilesWatcher Detects
- Type is `kubernetes.io/basic-auth`
- Adds to state map: `state["my-proxy"]`
- Calculates checksum of secret data
- Triggers router config update

### Step 3: Disk Sync
- Creates directory: `/etc/skupper-router-proxies/my-proxy/`
- Writes file: `/etc/skupper-router-proxies/my-proxy/password.txt` containing password

### Step 4: Create Link
```yaml
apiVersion: skupper.io/v2alpha1
kind: Link
metadata:
  name: to-east
  namespace: west
spec:
  endpoints:
    - name: inter-router
      host: east.example.com
      port: "55671"
  tlsCredentials: link-tls
  settings:
    proxy-configuration: my-proxy  # References the Secret
```

### Step 5: ProxyConfig Creation
```go
// site.getProxyConfig("my-proxy") returns:
&site.ProxyConfig{
    Host:        "proxy.example.com",
    Port:        "8080",
    User:        "user1",
    ProfilePath: "/etc/skupper-router-proxies",
}
```

### Step 6: ProxyProfile Generation
```go
// ConfigureProxyProfile called with:
// name="my-proxy"
// host="proxy.example.com"
// port="8080"
// username="user1"
// path="/etc/skupper-router-proxies"

// Returns:
ProxyProfile{
    Name:     "my-proxy",
    Host:     "proxy.example.com",
    Port:     "8080",
    Username: "user1",
    Password: "file:/etc/skupper-router-proxies/my-proxy/password.txt",
}
```

### Step 7: Router Config
```json
{
  "proxyProfiles": [
    {
      "name": "my-proxy",
      "host": "proxy.example.com",
      "port": "8080",
      "username": "user1",
      "password": "file:/etc/skupper-router-proxies/my-proxy/password.txt"
    }
  ],
  "connectors": [
    {
      "name": "to-east",
      "role": "inter-router",
      "host": "east.example.com",
      "port": "55671",
      "sslProfile": "link-tls-profile",
      "proxyProfile": "my-proxy"
    }
  ]
}
```

### Step 8: Router Sync
- Agent creates ProxyProfile entity in router
- Connector references it by name
- Router reads password from file when connecting

## 9. Key Design Patterns

### Separation of Concerns
1. **Secret Storage**: Kubernetes Secret (credentials)
2. **Disk Persistence**: Password file (runtime access)
3. **Config Reference**: Link YAML (logical association)
4. **Router Config**: JSON (entity definitions)

### Security Considerations
1. **Password on Disk**: Avoids password in logs/config dumps
2. **File Permissions**: 0644 readable by router process
3. **No Password in ConfigMap**: Router config in ConfigMap doesn't contain password
4. **Secret Type Enforcement**: Must be `kubernetes.io/basic-auth`

### Change Propagation
```
Secret Update
    ↓
ProfilesWatcher detects change (checksum)
    ↓
Sync.handleProxyProfile writes to disk
    ↓
Callback triggers config recheck
    ↓
ConfigSync.configEvent processes ConfigMap
    ↓
ExpectProxyProfiles validates profiles
    ↓
SyncProxyProfilesToRouter updates router
```

### Cache-First Retrieval
- Secrets cached by ProfilesWatcher
- `s.profiles.Cache.Get()` for fast lookup
- No K8s API call during Link processing

## 10. Non-Kube Translation Considerations

For implementing proxy configuration in non-Kubernetes environments:

### Replace Secret with Config File
- **K8s**: Secret with base64 data
- **Non-Kube**: JSON/YAML config file with plain credentials

### Replace Watcher with File Monitor
- **K8s**: ProfilesWatcher on Secret resources
- **Non-Kube**: fsnotify or inotify on config files

### Replace Cache with In-Memory Map
- **K8s**: Informer cache
- **Non-Kube**: Map with file path → proxy config

### Disk Layout Unchanged
- Keep `/etc/skupper-router-proxies/{name}/password.txt`
- Same file permissions
- Same `file:` prefix in ProxyProfile.Password

### Config File Example (Non-Kube)
```json
{
  "proxies": {
    "my-proxy": {
      "host": "proxy.example.com",
      "port": "8080",
      "username": "user1",
      "password": "<your-password>"
    }
  }
}
```

### Link Reference (Non-Kube)
```json
{
  "links": [
    {
      "name": "to-east",
      "endpoints": [
        {"host": "east.example.com", "port": 55671}
      ],
      "tlsCredentials": "link-tls",
      "proxyConfiguration": "my-proxy"
    }
  ]
}
```

## 11. File Reference Quick Map

| Component | File Path | Lines | Purpose |
|-----------|-----------|-------|---------|
| Secret Example | `/internal/kube/secrets/sync_test.go` | 232-247 | Test fixture showing secret structure |
| Secret Detection | `/internal/kube/secrets/manager.go` | 122-140 | ProfilesWatcher detects basic-auth secrets |
| Disk Write | `/internal/kube/secrets/sync.go` | 348-361 | writeProxyProfile creates password.txt |
| Profile Handling | `/internal/kube/secrets/sync.go` | 123-151 | handleProxyProfile orchestrates sync |
| Profile Validation | `/internal/kube/secrets/sync.go` | 260-286 | ExpectProxyProfiles validates and prepares |
| Link API | `/pkg/apis/skupper/v2alpha1/types.go` | 584-596 | LinkSpec and GetProxyConfiguration |
| Secret Retrieval | `/internal/kube/site/site.go` | 1312-1326 | getProxyConfig fetches from cache |
| Link Application | `/internal/site/link.go` | 40-78 | Link.Apply creates ProxyProfile |
| Profile Builder | `/internal/qdr/qdr.go` | 220-230 | ConfigureProxyProfile builds struct |
| Profile Struct | `/internal/qdr/qdr.go` | 588-594 | ProxyProfile definition |
| Router Sync | `/internal/qdr/sync_router_ops.go` | 130-164 | SyncProxyProfilesToRouter updates router |
| Config Sync | `/internal/kube/adaptor/config_sync.go` | 92-122 | configEvent orchestrates all syncs |

## Summary

The K8s implementation uses:
1. **Kubernetes Secret** (type `basic-auth`) for credential storage
2. **ProfilesWatcher** for automatic secret detection
3. **Disk files** (`/etc/skupper-router-proxies/{name}/password.txt`) for password isolation
4. **Link.Settings** for logical proxy-to-link association
5. **ProxyProfile** JSON entity in router config
6. **AMQP agent** for router synchronization

Key insight: Password stored on disk and referenced via `file:` prefix, while host/port/username go directly in ProxyProfile JSON.
