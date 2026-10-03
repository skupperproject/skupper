# Redis Multicloud High Availability Test

This README provides an overview of the "Redis Multicloud High Availability Test" Ansible playbook for testing Skupper connectivity across multiple Kubernetes clusters with a distributed Redis setup.

## Overview

The Redis Multicloud HA Test demonstrates a highly available Redis architecture with Sentinel across multiple Kubernetes clusters (named "west", "east", and "north") plus a Podman environment, all connected via Skupper. It creates Redis Server and Sentinel deployments across multiple sites and establishes Skupper connectivity between them to verify distributed caching and failover capabilities.

## Prerequisites

- Three Kubernetes clusters (identified as "west", "east", and "north" in your inventory)
- A Podman environment
- Ansible with the following collections installed:
  - `e2e.tests`
  - `skupper.v2`
  - `kubernetes.core`
  - `containers.podman`
- Appropriate kubeconfig files with access to all clusters

## Playbook Structure

The playbook executes the following sequence of operations:

1. **Kubernetes Environment Setup**
   - Performs an environment check via the `e2e.tests.env_shakeout` role
   - Generates namespaces using the `e2e.tests.generate_namespaces` role
   - Creates Skupper sites in each namespace
   - Waits for Skupper site pods to be in Running state

2. **Redis Deployment**
   - Deploys Redis Server and Sentinel in each Kubernetes cluster
   - Waits for Redis Server pods to be in Running state
   - Creates Listeners and Connectors resources in each namespace
   - Waits for Redis Sentinel pods to be in Running state

3. **Site Connectivity**
   - Creates a Skupper access link from the west namespace
   - Applies the token to the east and north sites
   - Waits for site connectivity to be established

4. **Podman Environment Setup**
   - Creates a Skupper podman network
   - Creates Skupper sites resources in the Podman environment
   - Applies the access token to the Podman site
   - Initializes the default namespace using Podman

5. **Teardown (Optional)**
   - Performs cleanup of all resources if the teardown flag is set
   - Deletes namespaces in Kubernetes environments
   - Performs teardown in the Podman environment

## Resource Files

The playbook uses various resource definition files hosted in a GitHub repository:

### West Cluster
- `site-west.yaml` - Skupper site configuration
- `redis-west.yaml` - Redis server and sentinel configuration
- `listener-west.yaml` - Skupper listener configuration
- `connector-west.yaml` - Skupper connector configuration

### East Cluster
- `site-east.yaml` - Skupper site configuration
- `redis-east.yaml` - Redis server and sentinel configuration
- `listener-east.yaml` - Skupper listener configuration
- `connector-east.yaml` - Skupper connector configuration

### North Cluster
- `site-north.yaml` - Skupper site configuration
- `redis-north.yaml` - Redis server and sentinel configuration (primary Redis server)
- `listener-north.yaml` - Skupper listener configuration
- `connector-north.yaml` - Skupper connector configuration

### Podman Environment
- `listener-podman.yaml` - Skupper listener configuration for Podman
- `site-podman.yaml` - Skupper site configuration for Podman

## Variables

### Global Variables (group_vars/all.yml)

```yaml
ansible_connection: local
ansible_user: "{{ lookup('env', 'USER') }}"
debug: false
namespace_prefix: "e2e"
teardown_flag: true
```

### Host-specific Variables

#### West Cluster (host_vars/west.yml)

```yaml
# Kubeconfig path for west site
kubeconfig_1: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_1 }}"

# Namespace configuration
namespace_name: redis-west

# West CRs
site: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/west-crs/site-west.yaml"
connector: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/west-crs/connector-west.yaml"
listener: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/west-crs/listener-west.yaml"
redis: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/west-crs/redis-west.yaml"
```

#### East Cluster (host_vars/east.yml)

```yaml
# Kubeconfig path for east site
kubeconfig_2: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_2 }}"

# Namespace configuration
namespace_name: redis-east

# East CRs
site: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/east-crs/site-east.yaml"
connector: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/east-crs/connector-east.yaml"
listener: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/east-crs/listener-east.yaml"
redis: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/east-crs/redis-east.yaml"
```

#### North Cluster (host_vars/north.yml)

```yaml
# Kubeconfig path for north site
kubeconfig_3: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_3 }}"

# Namespace configuration
namespace_name: redis-north

# North CRs
site: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/north-crs/site-north.yaml"
connector: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/north-crs/connector-north.yaml"
listener: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/north-crs/listener-north.yaml"
redis: "https://raw.githubusercontent.com/skupperproject/skupper-example-redis/refs/heads/main/north-crs/redis-north.yaml"
```

### Other Key Variables

- `teardown_flag` - Boolean flag to control resource cleanup (default: true)

## Inventory Structure

The test uses a specific inventory structure:

```
redis/
└── inventory/
    ├── hosts.yml                 # Defines the west, east, north and podman hosts
    ├── group_vars/
    │   └── all.yml              # Global variables for all hosts
    └── host_vars/
        ├── west.yml             # West-specific variables
        ├── east.yml             # East-specific variables
        ├── north.yml            # North-specific variables
        └── podman/              # Podman-specific variables
```

The hosts.yml file defines the four required hosts:

```yaml
---
all:
  hosts:
    west:
    east:
    north:
    podman:
```

## Usage

Run the playbook with the provided inventory:

```bash
ansible-playbook redis_test.yml -i inventory/
```

To skip the teardown phase:

```bash
ansible-playbook redis_test.yml -i inventory/ -e teardown_flag=false
```

You can override the kubeconfig paths by passing them as extra variables:

```bash
ansible-playbook redis_test.yml -i inventory/ \
  -e kubeconfig_1=/path/to/west/kubeconfig \
  -e kubeconfig_2=/path/to/east/kubeconfig \
  -e kubeconfig_3=/path/to/north/kubeconfig
```

## Test Flow

1. The playbook creates isolated namespaces on all three Kubernetes clusters
2. It sets up Skupper sites in all clusters and the Podman environment
3. It deploys Redis Server and Sentinel across the sites, with the primary server in the north cluster
4. It establishes Skupper connectivity between all sites
5. Redis replication and Sentinel monitoring is set up across all clusters
6. The Podman environment is connected to provide a client interface
7. All resources are cleaned up after the test completes (unless skipped)

## Architecture

The Redis Multicloud HA test creates the following architecture:

- **North Site**: Hosts the primary Redis server
- **East & West Sites**: Host Redis replica servers
- **All Sites**: Run Redis Sentinel for monitoring and automatic failover
- **Podman Environment**: Provides a client interface to interact with the Redis cluster

Skupper connects all these components securely without exposing them to the public internet, enabling:

1. Replica synchronization across clusters
2. Sentinel monitoring across clusters
3. Automatic failover if the primary server fails
4. Client connectivity from any site to the current primary

## Troubleshooting

- Verify that all resource files exist and are accessible from the GitHub repository
- Check the Skupper console and logs if connectivity issues occur
- Ensure the kubeconfig files have appropriate permissions to create resources in all clusters
- Validate that Podman is properly configured and the API service is running
- Check the Redis Server and Sentinel logs for replication or monitoring issues
