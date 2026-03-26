# Implementation Plan: Debug Dump for Non-k8s Sites

## Overview
Implement `skupper debug dump <filename>` for non-k8s sites (podman, docker, linux platforms) to collect diagnostic information similar to the kubernetes implementation, but adapted for container/systemd-based deployments.

## Current State
- ✅ Command structure exists in `internal/cmd/skupper/debug/debug.go`
- ✅ Kube implementation complete in `internal/cmd/skupper/debug/kube/debug.go`
- ❌ Nonkube stub exists in `internal/cmd/skupper/debug/nonkube/debug.go` (only prints "not yet implemented")

## Architecture Understanding

### Data Storage Locations
- Root: `/var/lib/skupper/namespaces/<namespace>/`
- Non-root: `~/.local/share/skupper/namespaces/<namespace>/`

### Directory Structure
- `input/resources/` - User-configured resources (YAML files)
- `runtime/resources/` - Runtime state after bootstrap
- `runtime/router/` - Router configuration
- `runtime/certs/` - Certificates and issuers
- `internal/scripts/` - Systemd service files
- `internal/snapshot/` - Loaded site state

### Platform Types
- `podman` - Containers managed via Podman
- `docker` - Containers managed via Docker
- `linux` - Native processes managed via systemd

## Implementation Steps

### Step 1: Update CmdDebug Structure
**File:** `internal/cmd/skupper/debug/nonkube/debug.go`

Add required fields:
- `namespace string` - Target namespace (use lowercase for consistency)
- `fileName string` - Output tarball name
- `platform string` - Runtime platform (podman/docker/linux)
- `siteHandler *fs.SiteHandler` - Access site resources
- `connectorHandler *fs.ConnectorHandler` - Access connector resources
- `listenerHandler *fs.ListenerHandler` - Access listener resources
- `linkHandler *fs.LinkHandler` - Access link resources
- `routerAccessHandler *fs.RouterAccessHandler` - Access router access resources
- `certificateHandler *fs.CertificateHandler` - Access certificate resources
- `secretHandler *fs.SecretHandler` - Access secret resources (if needed)
- `configMapHandler *fs.ConfigMapHandler` - Access configmap resources (router config)

### Step 2: Implement ValidateInput

Validate:
- Filename argument (0-1 args, valid path, default: "skupper-dump")
- Site exists in namespace
- Can access site directory
- Platform detection from `internal/platform.yaml`

### Step 3: Implement InputToOptions

- Add timestamp to filename: `<filename>-<namespace>-<datetime>`
- Detect platform from namespace config
- Initialize container client (if podman/docker) or systemd interface (if linux)

### Step 4: Implement Run - Core Logic

Create tarball structure consistent with k8s implementation but adapted for non-k8s:

```
<filename>.tar.gz
├── versions/
│   ├── skupper.yaml           # skupper version output
│   ├── skupper.yaml.txt       # (duplicate for consistency)
│   ├── platform.yaml          # platform version (podman/docker/systemd)
│   └── platform.yaml.txt      # (duplicate for consistency)
├── site-namespace/            # Using same structure as k8s dump
│   ├── resources/
│   │   ├── Site-*.yaml
│   │   ├── Site-*.yaml.txt
│   │   ├── Connector-*.yaml
│   │   ├── Connector-*.yaml.txt
│   │   ├── Listener-*.yaml
│   │   ├── Listener-*.yaml.txt
│   │   ├── Link-*.yaml
│   │   ├── Link-*.yaml.txt
│   │   ├── Certificate-*.yaml
│   │   ├── Certificate-*.yaml.txt
│   │   ├── AccessToken-*.yaml
│   │   ├── AccessToken-*.yaml.txt
│   │   ├── RouterAccess-*.yaml
│   │   ├── RouterAccess-*.yaml.txt
│   │   ├── SecuredAccess-*.yaml
│   │   ├── SecuredAccess-*.yaml.txt
│   │   ├── Configmap-*.yaml        # router config as ConfigMap equivalent
│   │   ├── Configmap-*.yaml.txt
│   │   ├── Container-*.json        # (podman/docker) container inspect
│   │   ├── Container-*.json.txt    # (podman/docker)
│   │   ├── Systemd-*.txt           # (linux) systemd service status
│   │   ├── platform.yaml           # Platform configuration
│   │   └── skstat/                 # Router statistics (same location as k8s)
│   │       ├── router-skstat-g.txt
│   │       ├── router-skstat-c.txt
│   │       ├── router-skstat-l.txt
│   │       ├── router-skstat-n.txt
│   │       ├── router-skstat-e.txt
│   │       ├── router-skstat-a.txt
│   │       ├── router-skstat-m.txt
│   │       └── router-skstat-p.txt
│   ├── logs/
│   │   ├── router.txt              # Router logs
│   │   ├── controller.txt          # Controller logs (if applicable)
│   │   └── systemd-journal.txt     # (linux only) journalctl output
│   └── containers.json             # (podman/docker) list of all containers
```

**Key consistency points with k8s dump:**
- Duplicate files with `.txt` extension alongside YAML/JSON for easier viewing
- Use `site-namespace/` as main directory (matches k8s structure)
- Place `skstat/` under `resources/` (matches k8s pattern)
- Keep `logs/` at namespace level
- All resources in `resources/` directory

### Step 5: Information Collection Functions

#### 5.1 Version Information
- Run `skupper version -o yaml`
- Collect platform info (podman/docker version, systemd version, OS details)
- Write to `/versions/`

#### 5.2 Site Resources
- Read all YAML files from `<datapath>/input/resources/`
- Read all YAML files from `<datapath>/runtime/resources/`
- Use existing `fs.*Handler` classes (SiteHandler, ConnectorHandler, ListenerHandler, etc.)
- Write each resource as YAML to `/site-namespace/resources/`
- **Important:** Write both `.yaml` and `.yaml.txt` versions of each file (k8s consistency)

#### 5.3 Router Configuration
- Read router config from `<datapath>/runtime/router/`
- Write as `Configmap-skupper-router.yaml` to `/site-namespace/resources/`
- Include both `.yaml` and `.yaml.txt` versions
- This mirrors how k8s stores router config in a ConfigMap

#### 5.4 Platform-Specific Info

**For Podman/Docker:**
- Use `internal/nonkube/client/compat` container client
- `ContainerList()` - List all skupper containers (filter by label `application=skupper`)
  - Write to `/site-namespace/containers.json`
- `ContainerInspect()` - Detailed info for each router/controller container
  - Write as `Container-<name>.json` and `Container-<name>.json.txt` to `/site-namespace/resources/`
- `ContainerLogs()` - Retrieve container logs
  - Write to `/site-namespace/logs/<container-name>.txt`
  - Use container name in filename (e.g., `skupper-router-default.txt`)
- **Note:** Container names replace pod names from k8s implementation

**For Linux:**
- Use systemd commands via `internal/nonkube/common/systemd.go`
- `systemctl status skupper-<namespace>.service`
  - Write to `/site-namespace/resources/Systemd-skupper-<namespace>.txt`
- Copy systemd service file from `<datapath>/internal/scripts/`
  - Write to `/site-namespace/resources/Systemd-service-file.txt`
- `journalctl -u skupper-<namespace>.service` for logs
  - Write to `/site-namespace/logs/systemd-journal.txt`

#### 5.5 Router Statistics
- If router container/process is running, execute `skstat` commands:
  - `skstat -g` (general)
  - `skstat -c` (connections)
  - `skstat -l` (links)
  - `skstat -n` (nodes)
  - `skstat -e` (edge routers)
  - `skstat -a` (addresses)
  - `skstat -m` (memory)
  - `skstat -p` (priorities)
- For containers: use `ContainerExec(containerName, []string{"skstat", "-<flag>"})`
  - Find router container from ContainerList() (look for router in name/image)
  - Write as `/site-namespace/resources/skstat/<container-name>-skstat-<flag>.txt`
- For linux: execute skstat via the router service
  - May need to exec into router namespace/environment
  - Write as `/site-namespace/resources/skstat/skupper-<namespace>-skstat-<flag>.txt`
- **Note:** Matches k8s location pattern of `resources/skstat/`, using container/service name instead of pod name

### Step 6: Helper Functions

**Use existing utilities from `internal/cmd/skupper/common/utils/debug.go`:**
- `utils.RunCommand(name, args...)` - Execute external commands (already exists)
- `utils.WriteTar(name, data, timestamp, tarWriter)` - Add file to tarball (already exists)
- `utils.WriteObject(object, baseName, tarWriter)` - Write both `.yaml` and `.yaml.txt` (already exists)

**Create new helper methods in debug.go:**
- `collectSiteResources(tw)` - Gather all CR files from fs handlers
- `collectContainerInfo(tw)` - Podman/Docker container list and inspect details
- `collectSystemdInfo(tw)` - Systemd service status and configuration
- `collectRouterConfig(tw)` - Router configuration files
- `collectLogs(tw)` - Platform-specific log collection (container logs or journalctl)
- `collectRouterStats(tw)` - Execute skstat commands if router is running
- `collectVersionInfo(tw)` - Skupper version and platform info
- `collectCertificates(tw)` - Certificate files from input/runtime paths

### Step 7: Error Handling

- Continue collection on individual failures (don't fail entire dump)
- Log warnings for missing components
- Include error information in tarball if collection fails

### Step 8: Testing

Create test file: `internal/cmd/skupper/debug/nonkube/debug_test.go`

Test cases:
- Filename validation
- Default filename generation
- Tarball creation with timestamp
- Resource collection from filesystem
- Platform detection
- Container client integration (mocked)
- Systemd integration (mocked)

## Dependencies

Required imports:
- `archive/tar`
- `compress/gzip`
- `github.com/skupperproject/skupper/internal/nonkube/client/fs`
- `github.com/skupperproject/skupper/internal/nonkube/client/compat`
- `github.com/skupperproject/skupper/internal/nonkube/common`
- `github.com/skupperproject/skupper/pkg/nonkube/api`
- Standard library: `os`, `path`, `time`, `fmt`, `errors`

## Files to Create/Modify

**Modify:**
1. `internal/cmd/skupper/debug/nonkube/debug.go` - Main implementation (~400-500 lines)

**Create:**
2. `internal/cmd/skupper/debug/nonkube/debug_test.go` - Unit tests (~200-300 lines)

**Create (from PR #2174 pattern, not yet merged):**
3. `internal/cmd/skupper/common/utils/debug.go` - Shared utility functions
   - `RunCommand(name, args...)` - Execute external commands
   - `WriteTar(name, data, timestamp, tarWriter)` - Add file to tarball
   - `WriteObject(object, baseName, tarWriter)` - Write both `.yaml` and `.yaml.txt`
   - These will be shared between kube and nonkube implementations

## Success Criteria

- ✅ Command runs successfully on podman/docker/linux platforms
- ✅ Creates compressed tarball with timestamp
- ✅ Collects all site resources (Sites, Connectors, Listeners, Links, etc.)
- ✅ Includes router configuration and certificates
- ✅ Captures container/systemd status and logs
- ✅ Includes router statistics when available
- ✅ Handles missing components gracefully
- ✅ Output format consistent with kube implementation
- ✅ Tests provide adequate coverage

## Implementation Order

1. ✅ Basic structure and validation (Steps 1-3) - COMPLETED
   - Need to add missing resource handlers to Step 1
2. Create utility functions (Step 6a - `debug.go` utilities)
3. Tarball creation framework (Step 4 - skeleton)
4. Version info collection (Step 5.1)
5. Resource collection (Step 5.2)
6. Router config collection (Step 5.3)
7. Container/systemd integration (Step 5.4)
8. Logs and statistics (Step 5.5)
9. Error handling and polish (Step 7)
10. Testing (Step 8)

## Notes

- **Updated after reviewing actual k8s dump:** Structure changed to match k8s implementation more closely
  - Use `site-namespace/` as main directory (not separate `site/`, `runtime/`, `logs/`)
  - Place `skstat/` under `resources/` (not separate stats directory)
  - Duplicate files with `.txt` extension for easier viewing
  - Flatten hierarchy to match k8s pattern

- **Learnings from PR #2174 (open PR attempting same task):**
  - ✅ Reuse existing utility functions from `internal/cmd/skupper/common/utils/debug.go`
  - ✅ Initialize all resource handlers (connector, listener, site, link, routerAccess, certificate, secret, configMap)
  - ✅ Good pattern for container handling (ContainerInspect, ContainerExec, ContainerLogs)
  - ✅ Proper systemd/journalctl handling with user flag awareness
  - ⚠️ PR uses `/input/`, `/runtime/`, `/internal/` structure - we're using `/site-namespace/` instead (better)
  - ⚠️ PR notes "Missing skstat commands" - we'll ensure complete skstat implementation
  - ⚠️ PR doesn't consistently use `.txt` duplicates - we will for consistency with k8s

- Maintain consistency with kube implementation where possible
- Adapt tarball structure for non-k8s specifics (containers vs pods, systemd vs deployments)
- Ensure it works for both root and non-root users
- Handle all three platforms: podman, docker, linux
- Consider file permissions (some files may require elevated privileges)

## Reference Files

### Key existing implementations to reference:
- `internal/cmd/skupper/debug/kube/debug.go` - Kubernetes implementation pattern
- `internal/nonkube/client/fs/site_handler.go` - Site resource access
- `internal/nonkube/client/compat/container.go` - Container client interface
- `internal/nonkube/common/systemd.go` - Systemd service management
- `pkg/nonkube/api/environment.go` - Path and environment utilities

### Actual K8s Dump Structure (for reference)

From examining an actual k8s debug dump (`~/dump.tar.gz`):

```
/versions/
├── kubernetes.yaml
├── kubernetes.yaml.txt
├── skupper.yaml
└── skupper.yaml.txt

/site-namespace/
├── events.txt
├── resources/
│   ├── Deployment-skupper-router.yaml
│   ├── Deployment-skupper-router.yaml.txt
│   ├── Pod-skupper-router-*.yaml
│   ├── Pod-skupper-router-*.yaml.txt
│   ├── Configmap-skupper-router.yaml
│   ├── Configmap-skupper-router.yaml.txt
│   ├── Configmap-skupper-network-status.yaml
│   ├── Services-*.yaml
│   ├── Endpoints-*.yaml
│   ├── Role-skupper-router.yaml
│   ├── RoleBinding-skupper-router.yaml
│   ├── ReplicaSet-*.yaml
│   ├── crds.txt
│   ├── Site-*.yaml / Site-*.yaml.txt
│   ├── Connector-*.yaml
│   ├── Listener-*.yaml
│   ├── Link-*.yaml
│   ├── Certificate-*.yaml
│   ├── AccessToken-*.yaml / Accessgrant-*.yaml
│   ├── RouterAccess-*.yaml
│   ├── SecuredAccess-*.yaml
│   └── skstat/
│       ├── <pod-name>-skstat-g.txt
│       ├── <pod-name>-skstat-c.txt
│       ├── <pod-name>-skstat-l.txt
│       ├── <pod-name>-skstat-n.txt
│       ├── <pod-name>-skstat-e.txt
│       ├── <pod-name>-skstat-a.txt
│       ├── <pod-name>-skstat-m.txt
│       └── <pod-name>-skstat-p.txt
└── logs/
    ├── <pod-name>-router.txt
    ├── <pod-name>-kube-adaptor.txt
    └── <pod-name>-kube-adaptor-previous.txt (if container restarted)
```

**Key observations:**
- Flat structure under `site-namespace/`
- Duplicate `.yaml.txt` files for easy viewing
- `skstat` output under `resources/skstat/`
- Previous container logs captured when restarts occurred
- Events at top level of namespace directory
