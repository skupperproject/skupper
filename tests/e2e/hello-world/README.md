# Hello World E2E Test

This test validates basic Skupper functionality by deploying a simple application across two sites and verifying connectivity.

## Test Overview

The Hello World test creates a multi-site application with the following components:

1. **Backend Site** (east): Runs the backend service
2. **Frontend Site** (west): Runs the frontend service that connects to the backend

The test validates that:
- Skupper sites can be created successfully
- Sites can be connected securely
- Services can be exposed across sites
- Cross-site communication works properly

## Directory Structure

```
hello-world/
├── ansible.cfg                 # Ansible configuration
├── collections/                # Ansible collections
│   ├── ansible_collections/    # Installed collections
│   └── requirements.yml        # Collection dependencies
├── inventory/                  # Inventory directory
│   ├── group_vars/             # Variables for all hosts
│   │   └── all.yml             # Common variables
│   ├── hosts.yml               # Host definitions
│   └── host_vars/              # Host-specific variables
│       ├── east.yml            # Backend site variables
│       └── west.yml            # Frontend site variables
├── requirements.txt            # Python dependencies
└── test.yml                    # Main test playbook
```

## Requirements

- **Kubernetes**: One or two Kubernetes clusters with Skupper V2 installed
- **Ansible**: Core Ansible packages (listed in requirements.txt)
- **Collections**: Required Ansible collections (listed in collections/requirements.yml)
- **Kubeconfig**: Valid kubeconfig file with access to the target clusters

## Ansible Roles and Modules Used

This test uses the following key roles from the `rhsiqe.skupper` collection:

1. **skupper_test_images**: Configures test images used in the deployment
2. **env_shakeout**: Validates the environment setup
3. **generate_namespaces**: Creates test namespaces
4. **deploy_workload**: Deploys the backend and frontend services
5. **skupper_site**: Creates Skupper sites in both namespaces
6. **create_connector**: Creates a connector at the east site
7. **create_listener**: Creates a service listener at the west site
8. **access_grant**: Configures access between sites
9. **link_site**: Links the sites together
10. **expose_service**: Exposes the backend service to the frontend
11. **run_curl**: Tests connectivity between frontend and backend
12. **teardown_test**: Cleans up all created resources

## How to Run

### 1. Install Dependencies

```bash
# Create a virtual environment
python3 -m venv .venv
source .venv/bin/activate

# Install Python dependencies
pip install -r requirements.txt

# Install Ansible collections
ansible-galaxy collection install -r collections/requirements.yml
```

### 2. Configure Test Environment

The test uses two logical hosts: `east` (backend) and `west` (frontend). By default, both point to the same kubeconfig file.

To test across different clusters, modify the kubeconfig paths in:
- `inventory/host_vars/east.yml`
- `inventory/host_vars/west.yml`

### 3. Run the Test

```bash
# Basic execution
ansible-playbook test.yml -i inventory/hosts.yml

# With specific kubeconfig (overrides inventory settings)
ansible-playbook test.yml -i inventory/hosts.yml -e "kubeconfig=/path/to/kubeconfig"

# Skip teardown (keeps resources for debugging)
ansible-playbook test.yml -i inventory/hosts.yml -e "skip_teardown=true"
```

## Test Workflow

1. **Setup**: Creates temporary directories and validates environment
2. **Namespace Creation**: Generates namespaces for both sites
3. **Workload Deployment**:
   - Deploys backend service (3 replicas) at the east site
   - Deploys frontend service at the west site
4. **Skupper Setup**:
   - Creates Skupper sites in both namespaces
   - Creates a connector at the east site
   - Creates a listener at the west site
   - Establishes secure link between sites
5. **Service Exposure**: Exposes the backend service to make it available at the west site
6. **Validation**: Tests connectivity by running a curl command from the west site to the backend service
7. **Cleanup**: Removes all created resources (unless skip_teardown is set)

## Key Configuration Parameters

### East Site (Backend)
- **Namespace**: `hello-world-east`
- **Workload**: Backend service (3 replicas)
- **Image**: Defined by `skupper_test_image_hello_world_backend`

### West Site (Frontend)
- **Namespace**: `hello-world-west`
- **Workload**: Frontend service
- **Image**: Defined by `skupper_test_image_hello_world_frontend`
- **Exposed Service**: Port 8080 (LoadBalancer type)
- **Test Endpoint**: `backend:8080/api/hello`

## Troubleshooting

### Common Issues

1. **Connection Failures**:
   - Verify kubeconfig is valid
   - Check if Skupper is properly installed in both clusters
   - Ensure network policies allow Skupper traffic

2. **Resource Creation Errors**:
   - Verify RBAC permissions are sufficient
   - Check for namespace conflicts

3. **Test Endpoint Unreachable**:
   - Ensure the backend service is running
   - Verify the Skupper network is properly connected
   - Check Skupper status in both namespaces

### Debugging

For deeper investigation, you can:

1. Skip teardown to examine resources:
   ```bash
   ansible-playbook test.yml -i inventory/hosts.yml -e "skip_teardown=true"
   ```

2. Increase verbosity for more detailed logs:
   ```bash
   ansible-playbook test.yml -i inventory/hosts.yml -vvv
   ```

3. Check Skupper status directly:
   ```bash
   skupper status -n hello-world-east
   skupper status -n hello-world-west
   ```