# skupper_test_images

This role manages image references for Skupper testing environments, providing consistent image sources across test suites.

## Requirements

* Ansible 2.14 or higher
* Access to the image repositories (e.g., quay.io)

## Role Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `skupper_test_image_hello_world_frontend` | `quay.io/skupper/hello-world-frontend:latest` | Frontend image for hello-world test |
| `skupper_test_image_hello_world_backend` | `quay.io/skupper/hello-world-backend:latest` | Backend image for hello-world test |
| `skupper_test_image_lanyard` | `quay.io/skupper/lanyard:latest` | Lanyard utility image used for testing |

Each variable can be overridden by setting the corresponding environment variable:
- `SKUPPER_TEST_IMAGE_HELLO_WORLD_FRONTEND`
- `SKUPPER_TEST_IMAGE_HELLO_WORLD_BACKEND`
- `SKUPPER_TEST_IMAGE_LANYARD`

## Dependencies

None

## Usage Examples

### Basic usage in a playbook

```yaml
- name: Run tests with default images
  hosts: all
  roles:
    - rhsiqe.skupper.skupper_test_images
  tasks:
    - name: Deploy hello world frontend
      kubernetes.core.k8s:
        definition:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: frontend
          spec:
            template:
              spec:
                containers:
                  - name: frontend
                    image: "{{ skupper_test_image_hello_world_frontend }}"
```

### Overriding images with environment variables

You can override the default images by setting environment variables before running your playbook:

```bash
export SKUPPER_TEST_IMAGE_HELLO_WORLD_FRONTEND="my-registry/hello-world-frontend:v1.2.3"
ansible-playbook my-test-playbook.yml
```

### Usage in test suites

This role is designed to be included at the beginning of test playbooks to ensure consistent image references:

```yaml
- name: E2E Test Suite
  hosts: all
  roles:
    - rhsiqe.skupper.skupper_test_images
  tasks:
    - name: Deploy test components
      # Your test tasks here, using the image variables
```

## Implementation Details

The role sets default values for all test images and checks for environment variables that should override these defaults. This provides flexibility for testing against different image versions or registries.

## License

Apache License, Version 2.0