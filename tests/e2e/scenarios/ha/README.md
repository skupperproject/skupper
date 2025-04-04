# Skupper HA (High Availability) Test

This README provides an overview of the "Skupper HA Test". 

## Overview

This test creates two Skupper sites with the HA feature enabled, this creates an additional instance of the skupper router in each skupper site.
A backend application is deployed in the east_ha namespace and exposed via Skupper in the west_ha namespace.
Once all skupper components are up, a k8s job is created using the Locust image to send a lot of HTTP POST requests to the backend endpoint.
While the locust job is running, we start killing the router pods, first the router1, then the router2, in a looping.
When the locust job ends, we get its logs and inspect the number of failures.

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
   - Create AccessToken and AccessGrant
   - Connect the two Skupper sites
   - Deploy the backend application and create the proper connector and listener

2. **Run the test**
   - Creates a k8s Job using the Locust image : mirror.gcr.io/locustio/locust
   - start sending HTTP POST requests to the backend endpoint
   - Start the script that take these actions in a looping: 
     - Start to kill the pod of the router1 in the namespace west_ha
       - Wait until a new pod is up for the router1 in west_ha
     - Start to kill the pod of the router2 in the namespace west_ha
       - Wait until a new pod is up for the router2 in west_ha
     - Wait until all Skupper links are up
   - Inspect the Locust job logs and get the number of requests sent and the number of failures
   - If the number of failures is smaller than 5% of the total of requests, the test PASS, otherwise, it FAILS
    

3. **Teardown**
   - Delete the k8s job
   - Delete the backend application
   - Delete all Skupper components : Sites, Listenners and Connectors
   - Delete the namespaces

## Resource Files

The playbook expects the following resource definition files:

### West_HA Cluster
- `resources/west_ha/listener.yml` - Skupper listener configuration
- `resources/west_ha/site.yml` - Skupper site configuration

### East_HA Cluster
- `resources/east/backend.yml` - Backend application deployment
- `resources/east/connector.yml` - Skupper connector configuration
- `resources/east/site.yml` - Skupper site configuration


## Templates

 The test uses two tmplates : 
- `kill-router-pods.sh.j2` - The script that will kill the router pods
- `locust-job.yaml.j2` - The k8s job that send HTTP POST requests to backend


## Variables

### Global Variables (group_vars/all.yml)

```yaml
ansible_connection: local
ansible_user: "{{ lookup('env', 'USER') }}"
debug: false
locust_runtime: "2m"
namespace_prefix: "e2e"
generate_namespaces_namespace_label: "e2e"
remove_namespaces: true
```

### Host-specific Variables

#### West_HA Cluster (host_vars/west_ha.yml)

```yaml
# Kubeconfig path for west-ha site
kubeconfig_1: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_1 }}"
namespace_name: "west-ha"
```

#### East_HA Cluster (host_vars/east_ha.yml)

```yaml
# Kubeconfig path for east-ha site
kubeconfig_2: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_2 }}"
namespace_name: "east-ha"
```

