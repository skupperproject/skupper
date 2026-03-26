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
- `siteHandler *fs.SiteHandler` - Access site resources
- `namespace string` - Target namespace
- `fileName string` - Output tarball name
- `platform string` - Runtime platform (podman/docker/linux)

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

Create tarball structure similar to kube but adapted for non-k8s:

```
<filename>.tar.gz
├── versions/
│   ├── skupper.yaml        # skupper version output
│   └── platform.yaml       # platform, OS, runtime info
├── site/
│   ├── resources/          # All CR YAML files
│   │   ├── Site-*.yaml
│   │   ├── Connector-*.yaml
│   │   ├── Listener-*.yaml
│   │   ├── Link-*.yaml
│   │   ├── Certificate-*.yaml
│   │   ├── AccessToken-*.yaml
│   │   ├── RouterAccess-*.yaml
│   │   └── SecuredAccess-*.yaml
│   ├── config/
│   │   └── router-config/  # Router configuration files
│   └── platform.yaml       # Platform configuration
├── runtime/
│   ├── containers/         # (podman/docker only)
│   │   ├── router-inspect.json
│   │   ├── controller-inspect.json
│   │   └── container-list.json
│   ├── systemd/            # (linux only)
│   │   ├── service-status.txt
│   │   └── service-file.txt
│   └── stats/
│       └── skstat-*.txt    # Router stats from running container/process
└── logs/
    ├── router.log          # Router logs
    ├── controller.log      # Controller logs (if applicable)
    └── systemd-journal.log # (linux only) journalctl output
```

### Step 5: Information Collection Functions

#### 5.1 Version Information
- Run `skupper version -o yaml`
- Collect platform info (podman/docker version, systemd version, OS details)
- Write to `/versions/`

#### 5.2 Site Resources
- Read all YAML files from `<datapath>/input/resources/`
- Read all YAML files from `<datapath>/runtime/resources/`
- Use existing `fs.*Handler` classes (SiteHandler, ConnectorHandler, ListenerHandler, etc.)
- Write each resource as YAML to `/site/resources/`

#### 5.3 Router Configuration
- Copy files from `<datapath>/runtime/router/`
- Write to `/site/config/router-config/`

#### 5.4 Platform-Specific Info

**For Podman/Docker:**
- Use `internal/nonkube/client/compat` container client
- `ContainerList()` - List all skupper containers
- `ContainerInspect()` - Detailed info for router/controller containers
- `ContainerLogs()` - Retrieve container logs
- Write container info to `/runtime/containers/`
- Write logs to `/logs/`

**For Linux:**
- Use systemd commands via `internal/nonkube/common/systemd.go`
- `systemctl status skupper-<namespace>.service`
- `journalctl -u skupper-<namespace>.service` for logs
- Write systemd info to `/runtime/systemd/`
- Write logs to `/logs/`

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
- For containers: use `ContainerExec()`
- For linux: execute skstat from router process
- Write to `/runtime/stats/`

### Step 6: Helper Functions

Create utility functions:
- `writeTar(name, data, timestamp, tarWriter)` - Add file to tarball
- `collectSiteResources()` - Gather all CR files
- `collectContainerInfo()` - Podman/Docker container details
- `collectSystemdInfo()` - Systemd service details
- `collectRouterConfig()` - Router configuration files
- `collectLogs()` - Platform-specific log collection
- `collectRouterStats()` - Execute skstat commands
- `detectPlatform(namespace)` - Read platform from config

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

1. Basic structure and validation (Steps 1-3)
2. Tarball creation framework (Step 4 - skeleton)
3. Resource collection (Step 5.2)
4. Platform detection and version info (Step 5.1, 5.6)
5. Container/systemd integration (Step 5.4)
6. Logs and statistics (Step 5.4, 5.5)
7. Router config collection (Step 5.3)
8. Error handling and polish (Step 7)
9. Testing (Step 8)

## Notes

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
