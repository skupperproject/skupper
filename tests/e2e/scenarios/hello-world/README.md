# Hello World Test

This README provides an overview of the "Hello World Test" Ansible playbook for testing Skupper connectivity between two Kubernetes clusters.

## Overview

The Hello World Test demonstrates a basic Skupper setup between two Kubernetes clusters (named "west" and "east"). It creates a frontend application in the west cluster and a backend application in the east cluster, then establishes Skupper connectivity between them to verify cross-cluster communication.

## Prerequisites

- Two Kubernetes clusters (identified as "west" and "east" in your inventory)
- Ansible with the following collections installed:
  - `e2e.tests`
  - `skupper.v2`
  - Required e2e test roles
- Appropriate kubeconfig files with access to both clusters

## Playbook Structure

The playbook executes the following sequence of operations:

1. **Setup**
   - Sets up a temporary directory for test artifacts
   - Performs an environment check via the `e2e.tests.env_shakeout` role
   - Generates namespaces using the `e2e.tests.generate_namespaces` role

2. **West Cluster Configuration**
   - Creates Skupper resources on the west cluster:
     - Frontend application (from `resources/west/frontend.yml`)
     - Skupper site (from `resources/west/site.yml`)
     - Skupper listener (from `resources/west/listener.yml`)
   - Issues a Skupper access token for the east cluster to connect with

3. **East Cluster Configuration**
   - Creates Skupper resources on the east cluster:
     - Backend application (from `resources/east/backend.yml`)
     - Skupper site (from `resources/east/site.yml`)
     - Skupper connector (from `resources/east/connector.yml`)
   - Applies the access token from the west cluster to establish connectivity

4. **Testing**
   - Verifies connectivity by running a curl test from the west cluster to the backend service in the east cluster
   - Stores test results in the temporary directory

5. **Cleanup**
   - Removes all test resources from both clusters using the `e2e.tests.teardown_test` role
   - Cleanup can be skipped by setting the `skip_teardown` variable to `true`

## Resource Files

The playbook expects the following resource definition files:

### West Cluster
- `resources/west/frontend.yml` - Frontend application deployment
- `resources/west/site.yml` - Skupper site configuration
- `resources/west/listener.yml` - Skupper listener configuration

### East Cluster
- `resources/east/backend.yml` - Backend application deployment
- `resources/east/site.yml` - Skupper site configuration
- `resources/east/connector.yml` - Skupper connector configuration

## Variables

### Global Variables (group_vars/all.yml)

```yaml
ansible_connection: local
ansible_user: "{{ lookup('env', 'USER') }}"
debug: false
namespace_prefix: "e2e"
generate_namespaces_namespace_label: "e2e"
teardown_test_namespace_label: generate_namespaces_namespace_label
site_access_token_path: ""
```

### Host-specific Variables

#### West Cluster (host_vars/west.yml)

```yaml
# Kubeconfig path for west site
kubeconfig_1: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_1 }}"

# Namespace configuration
namespace_name: hello-world-west

# Run curl configuration
run_curl_namespace: default
run_curl_address: "backend:8080/api/hello"
run_curl_image: "{{ skupper_test_images_lanyard }}"
run_curl_pod_name: curl-test
run_curl_retries: 30
run_curl_delay: 6
```

#### East Cluster (host_vars/east.yml)

```yaml
# Kubeconfig path for east site
kubeconfig_2: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_2 }}"

# Namespace configuration
namespace_name: hello-world-east
```

### Other Key Variables

- `skip_teardown` - Boolean flag to skip resource cleanup (default: false)

## Inventory Structure

The test uses a specific inventory structure:

```
hello-world/
└── inventory/
    ├── hosts.yml                 # Defines the west and east hosts
    ├── group_vars/
    │   └── all.yml              # Global variables for all hosts
    └── host_vars/
        ├── west.yml             # West-specific variables
        └── east.yml             # East-specific variables
```

The hosts.yml file simply defines the two required hosts:

```yaml
---
all:
  hosts:
    west:
    east:
```

## Usage

Run the playbook with the provided inventory:

```bash
ansible-playbook hello_world_test.yml -i hello-world/inventory/
```

To skip the teardown phase:

```bash
ansible-playbook hello_world_test.yml -i hello-world/inventory/ -e skip_teardown=true
```

You can override the kubeconfig paths by passing them as extra variables:

```bash
ansible-playbook hello_world_test.yml -i hello-world/inventory/ \
  -e kubeconfig_1=/path/to/west/kubeconfig \
  -e kubeconfig_2=/path/to/east/kubeconfig
```

## Test Flow

1. The playbook creates isolated namespaces on both clusters
2. It deploys a frontend application in the west cluster and a backend application in the east cluster
3. It sets up Skupper sites in both clusters and establishes a connection between them
4. The frontend in west can now communicate with the backend in east via Skupper
5. A curl test verifies this connectivity works as expected
6. All resources are cleaned up after the test completes (unless skipped)

## Troubleshooting

- Test results are stored in `/tmp/ansible.<hostname>` directory
- Check the Skupper console and logs if connectivity issues occur
- Verify that all resource files exist in the expected locations
- Ensure the kubeconfig files have appropriate permissions to create resources in both clusters
