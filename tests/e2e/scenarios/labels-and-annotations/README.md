# Skupper Custom Labels and Annotations Test

This README provides an overview of the "Skupper Custom Labels and Annotations Test"

## Overview

This test validates Skupper's ability to manage custom labels and annotations on components using a ConfigMap.

    Scenario 1: Pre-existing Component
    A component created before a ConfigMap with custom settings is deployed will be automatically labeled and annotated once the Skupper controller processes the ConfigMap.

    Scenario 2: New Component
    A component created while a ConfigMap with custom settings already exists will be created with the custom labels and annotations already applied.

    Scenario 3: Removal
    When the ConfigMap with custom settings is removed, all custom labels and annotations must be removed from components, regardless of when they were created.


## Prerequisites

- One Kubernetes clusters, where two namespaces will be used (identified as "label-annot-west" and "label-annot-east" in your inventory)
- Ansible with the following collections installed:
  - `e2e.tests`
  - `skupper.v2`
  - Required e2e test roles
- Appropriate kubeconfig files with access to both clusters


## Playbook Structure

The playbook executes the following sequence of operations:

1. **Initial Validation**
   - Verify that no ConfigMap with the designated custom labels and annotations exists in the cluster before starting the test.

2. **Setup**
   - Deploy a Skupper site in the label-annot-east namespace.
   - Apply a ConfigMap containing custom labels and annotations to the same namespace where the Skupper controller is running.

3. **Run the Test**
   - Confirm that the existing Deployment in label-annot-east (created **before** the ConfigMap was processed) now has the custom labels and annotations applied.
   - Create a new Skupper site in label-annot-west.
   - Confirm that the new Deployment in label-annot-west (created **after** the ConfigMap was processed) also has the custom labels and annotations applied.
   - Delete the ConfigMap.
   - Verify that the custom labels and annotations are successfully removed from both Deployments in label-annot-east and label-annot-west.

4. **Teardown**
   - Remove the Skupper sites created during the test.
   - Delete the namespaces used by the test.


## Resource Files

The playbook expects the following resource definition files:

### West Cluster
- `resources/west/00-ns.yaml` - The test namespace
- `resources/west/01-site-with-label-annot.yaml` - The site created after the ConfigMap with the custom labels and annotations

### East Cluster
- `resources/east/00-ns.yaml` - The test namespace
- `resources/east/01-site-no-label-annot.yaml` - The initial site, created before the ConfigMap with the custom labels and annotations
- `resources/east/03-cm-label-annot-all-kinds.yaml` - The ConfigMap with the custom labels and annotations


## Variables

### Global Variables (`group_vars/all.yml`)

```yaml
ansible_connection: local
ansible_user: "{{ lookup('env', 'USER') }}"
debug: false
namespace_prefix: "e2e"
generate_namespaces_namespace_label: "label-annot"
remove_namespaces: true
resource_retry_value: 30
resource_delay_value: 10
min_verbosity: 1
skupper_controller_ns: "skupper"

custom_annotations:
  com.acme.foo/annotest: "skupper-custom-annotation"
  sku.pper: "skupper-custom-annotation2"

custom_labels:
  acme.foo/labeltest : "skupper-custom-label"
  sku.pper : "skupper-custom-label2"
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
kubeconfig_1: "{{ ansible_env.HOME }}/.kube/config"
kubeconfig: "{{ kubeconfig_1 }}"
namespace_name: "east"
```

