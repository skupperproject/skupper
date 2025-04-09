# Skupper HA (High Availability) Test

This README provides an overview of the "Skupper HA Test."

## Overview

This test creates two Skupper sites with the HA feature enabled, which results in an additional instance of the Skupper router in each Skupper site.
A backend application is deployed in the `eastha` namespace and exposed via Skupper in the `westha` namespace.
Once all Skupper components are up, a Kubernetes job is created using the Locust image to send a large number of HTTP POST requests to the backend endpoint.

While the Locust job is running, we start terminating the router pods—first `router1`, then `router2`—in a loop.
Once the Locust job completes, we retrieve its logs and inspect the number of failures.

## Prerequisites

- Two Kubernetes clusters (identified as "westha" and "eastha" in your inventory)
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
   - Deploy the backend application and configure the appropriate connectors and listeners

2. **Run the test**
   - Create a Kubernetes job using the Locust image: `mirror.gcr.io/locustio/locust`
   - Start sending HTTP POST requests to the backend endpoint
   - Start a script that repeatedly performs the following actions:
     - Terminate the `router1` pod in the `westha` namespace
       - Wait until a new `router1` pod is running in `westha`
     - Terminate the `router2` pod in the `westha` namespace
       - Wait until a new `router2` pod is running in `westha`
     - Wait until all Skupper links are established
   - Inspect the Locust job logs and retrieve the number of requests sent and failures recorded
   - If the number of failures is less than 5% of the total requests, the test **passes**; otherwise, it **fails**

3. **Teardown**
   - Delete the Kubernetes job
   - Remove the backend application
   - Delete all Skupper components: Sites, Listeners, and Connectors
   - Delete the namespaces

## Resource Files

The playbook expects the following resource definition files:

### WestHA Cluster
- `resources/westha/listener.yml` - Skupper listener configuration
- `resources/westha/site.yml` - Skupper site configuration

### EastHA Cluster
- `resources/eastha/backend.yml` - Backend application deployment
- `resources/eastha/connector.yml` - Skupper connector configuration
- `resources/eastha/site.yml` - Skupper site configuration

## Templates

The test uses two templates:
- `kill-router-pods.sh.j2` - The script responsible for terminating router pods
- `locust-job.yaml.j2` - The Kubernetes job definition for sending HTTP POST requests to the backend

## Variables

### Global Variables (`group_vars/all.yml`)

```yaml
ansible_connection: local
ansible_user: "{{ lookup('env', 'USER') }}"
debug: false
locust_runtime: "2m"
namespace_prefix: "e2e"
generate_namespaces_namespace_label: "ha"
remove_namespaces: true
```

### Host-specific Variables

#### WestHA Cluster (`host_vars/westha.yml`)

```yaml
# Kubeconfig path for westha site
kubeconfig_1: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_1 }}"
namespace_name: "westha"
```

#### EastHA Cluster (`host_vars/eastha.yml`)

```yaml
# Kubeconfig path for eastha site
kubeconfig_2: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_2 }}"
namespace_name: "eastha"
```
