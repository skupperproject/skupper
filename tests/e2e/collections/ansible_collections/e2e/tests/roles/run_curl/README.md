# ansible-run_curl

This role deploys a temporary pod with curl capabilities in a Kubernetes namespace and executes HTTP requests, providing a way to test connectivity and validate HTTP endpoints within a cluster.

## Requirements

* Ansible 2.14 or higher
* Kubernetes cluster access via kubeconfig
* `kubernetes.core` collection

## Role Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `namespace_prefix` | Required | Prefix for the namespace name |
| `namespace_name` | Required | Base name of the namespace |
| `run_curl_image` | Required | Container image to use for the curl pod (e.g., `quay.io/skupper/lanyard`) |
| `run_curl_address` | Required | URL or endpoint to curl (e.g., `backend:8080/api/hello`) |
| `namespace` | Auto-generated | Full namespace name (`namespace_prefix-namespace_name`) |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `pod_retries` | `10` | Number of retries for pod deployment |
| `pod_delay` | `10` | Delay in seconds between pod deployment retries |
| `curl_retries` | `5` | Number of retries for curl command |
| `curl_delay` | `5` | Delay in seconds between curl command retries |

## Example Usage

```yaml
- name: Test API connectivity
  ansible.builtin.include_role:
    name: e2e.tests.run_curl
  vars:
    namespace_prefix: "e2e"
    namespace_name: "hello-world"
    run_curl_image: "quay.io/skupper/lanyard"
    run_curl_address: "backend:8080/api/hello"
```

## Role Behavior

1. Creates a concatenated namespace name using prefix and name
2. Deploys a pod with the specified image to execute curl commands
3. Runs a curl command against the specified address
4. Checks for a 200 HTTP response code
5. Returns the response body for validation
6. Outputs the curl results for debugging

## Notes

* The role will retry the pod deployment and curl execution based on the retry parameters
* The pod is deployed with a sleep command to keep it running for up to 1 hour
* The curl command is designed to handle both response body and status code validation
* The role outputs the response body to the console for debug purposes

## License

Apache License, Version 2.0
