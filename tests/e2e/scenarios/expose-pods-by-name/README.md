# Skupper Expose Pods By Name Test

This README provides an overview of the "Skupper Expose Pods By Name Test"

## Overview

This test creates two Skupper sites  and link them.
A backend application is deployed in the `east` namespace and exposed
via Skupper in the `west` namespace, with 3 replicas.
We have the option exposePodsByName set in both Connector and Listener

The test  perform the following steps : 
- Ensure that the exposed services matches the service pod names.
- Restart the service deployment. This will force new pod names replacing the old ones.
- Ensure that the exposed services matches only the new pod names.


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
   - Create Skupper sites
   - Generate AccessToken and AccessGrant
   - Connect the two Skupper sites
   - Deploy the backend application and the 3 replicas
   - Set the option exposePodsByName in the appropriate connectors and listeners

2. **Run the test**
   - Ensure that all Skupper sites are connected
   - Ensure that the exposed services matches the service pod names
   - Restart the backend deployment
   - Ensure that the exposed services matches the service pod names, specifically if the old pods were removed from services list.
   - If the exposed services names matches exactly the number and name of backend pods, the test PASS

3. **Teardown**
   - Delete the Kubernetes job
   - Remove the backend application
   - Delete all Skupper components: Sites, Listeners, and Connectors
   - Delete the namespaces

## Resource Files

The playbook expects the following resource definition files:

### West Cluster
- `resources/west/listener.yml` - Skupper listener configuration
- `resources/west/site.yml` - Skupper site configuration

### East Cluster
- `resources/east/backend.yml` - Backend application deployment
- `resources/east/connector.yml` - Skupper connector configuration
- `resources/east/site.yml` - Skupper site configuration

## Variables

### Global Variables (`group_vars/all.yml`)

```yaml
ansible_connection: local
ansible_user: "{{ lookup('env', 'USER') }}"
debug: false
namespace_prefix: "e2e"
generate_namespaces_namespace_label: "xpbn"
remove_namespaces: true
```

### Host-specific Variables

#### West Cluster (`host_vars/west.yml`)

```yaml
# Kubeconfig path for west site
kubeconfig_1: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_1 }}"
namespace_name: "west"
```

#### East Cluster (`host_vars/east.yml`)

```yaml
# Kubeconfig path for east site
kubeconfig_2: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_2 }}"
namespace_name: "east"
```
