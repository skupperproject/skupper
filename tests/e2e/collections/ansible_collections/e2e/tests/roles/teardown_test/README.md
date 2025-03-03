# ansible-teardown_test

This role deletes Kubernetes namespaces based on a label selector and removes a specified temporary directory.

## Requirements

* Ansible 2.14 or higher
* Kubernetes cluster access via kubeconfig

## Variables

| Variable                       | Default | Description                                 |
|--------------------------------|---------|---------------------------------------------|
| `teardown_test_namespace_label` | `""`    | Label used to identify namespaces for deletion. |
| `teardown_test_temp_dir_path`   | `""`    | Path to the temporary directory to remove. |
| `kubeconfig`                   | `null`  | Path to the kubeconfig file. (Required)     |

## Examples of usage

```yaml
- hosts: localhost
  tasks:
    - name: Teardown test environment
      include_role:
        name: teardown_test
      vars:
        teardown_test_namespace_label: "test"
        teardown_test_temp_dir_path: "/tmp/test-data"
        kubeconfig: "~/.kube/config"
```

## License

Apache License, Version 2.0

