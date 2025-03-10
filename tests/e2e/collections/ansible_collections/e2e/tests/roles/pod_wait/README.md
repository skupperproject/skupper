# ansible-pod_wait

This role waits for all pods in a specified Kubernetes namespace to reach the 'Running' state, providing a reliable way to ensure pod readiness before proceeding with subsequent tasks in your playbooks.

## Requirements

* Ansible 2.14 or higher
* Kubernetes cluster access via kubeconfig
* `kubernetes.core` collection

## Role Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `namespace_prefix` | Required | Prefix for the namespace name |
| `namespace_name` | Required | Base name of the namespace |
| `kubeconfig` | Required | Path to the kubeconfig file for cluster access |
| `pod_wait_label_selectors` | `""` | Label selectors to filter which pods to wait for |
| `pod_wait_retries` | `30` | Number of retries for checking pod status |
| `pod_wait_delay` | `6` | Delay in seconds between status check retries |

## Example Usage

```yaml
- name: Wait for all pods to be in Running state
  ansible.builtin.include_role:
    name: e2e.tests.pod_wait
  vars:
    namespace_prefix: "e2e"
    namespace_name: "hello-world"
    kubeconfig: "/path/to/kubeconfig"
    pod_wait_label_selectors: "app=myapp"
```

## Role Behavior

1. Constructs the full namespace name by combining `namespace_prefix` and `namespace_name`
2. Queries the Kubernetes API for pods in the specified namespace matching the provided label selectors
3. Verifies that at least one pod exists and all pods are in the 'Running' state
4. Retries the check according to the configured retry parameters
5. Succeeds when all matching pods are in the 'Running' state, or fails after exhausting all retries

## Notes

* This role is useful for ensuring application readiness before executing tests or subsequent deployment steps
* The role will wait for a total of approximately `pod_wait_retries * pod_wait_delay` seconds before failing
* If no label selectors are provided, the role will wait for all pods in the namespace
* For complex pod readiness checks, consider extending this role with additional status conditions

## License

Apache License, Version 2.0
