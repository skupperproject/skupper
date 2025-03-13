# ansible-generate_namespaces

This role creates Kubernetes namespaces with a configurable prefix and label.

## Requirements

* Ansible 2.14 or higher
* Kubernetes cluster access via kubeconfig

## Variables

| Variable                       | Default | Description                                 |
|--------------------------------|---------|---------------------------------------------|
| `generate_namespaces_namespace_label` | `test`  | Label applied to the created namespace.    |
| `namespace_prefix`             | `null`  | Prefix for the namespace name. (Required)   |
| `namespace_name`               | `null`  | Name of the namespace. (Required)          |
| `kubeconfig`                   | `null`  | Path to the kubeconfig file. (Required)     |

## Examples of usage

```yaml
- hosts: localhost
  tasks:
    - name: Create namespaces
      include_role:
        name: generate_namespaces
      vars:
        namespace_prefix: "my-app"
        namespace_name: "dev"
        kubeconfig: "~/.kube/config"
```

## License

Apache License, Version 2.0

