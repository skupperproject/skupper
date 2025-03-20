# iPerf3 Attached Connector E2E Test

This test evaluates network connectivity and performance between different components using iPerf3 and Skupper Attached Connectors.

## Test Overview

![Diagram](diagram.png)

The iPerf3 Attached Connector test validates Skupper's ability to connect services across different namespaces using the Attached Connector feature. The test environment consists of:

1. **Hub Site**: Central Skupper site that manages connections
2. **Client Site**: Runs the iPerf3 client that initiates performance tests
3. **Workload Site**: Contains the iPerf3 server that responds to client requests

The test verifies that:
- Skupper sites can be created in all namespaces
- Attached Connectors can link services across namespaces
- Network connectivity works through the Skupper network
- Performance metrics can be measured between endpoints

## Directory Structure

```
iperf3-attached/
├── ansible.cfg                 # Ansible configuration
├── collections/                # Ansible collections
│   └── requirements.yml        # Collection dependencies
├── inventory/                  # Inventory directory
│   ├── group_vars/             # Variables for all hosts
│   │   └── all.yml             # Common variables
│   ├── hosts.yml               # Host definitions
│   └── host_vars/              # Host-specific variables
│       ├── iperf3-client.yml   # Client site variables
│       ├── iperf3-hub.yml      # Hub site variables
│       └── iperf3-workload.yml # Workload site variables
├── resources/                  # Kubernetes and Skupper resource definitions
│   ├── iperf3-client/          # Client site resources
│   │   ├── iperf3-consumer.yaml # iPerf3 client job
│   │   ├── listener.yml        # Skupper listener definition
│   │   └── site.yml            # Skupper site definition
│   ├── iperf3-hub/             # Hub site resources
│   │   ├── attached-connector-binding.yml # Connector binding
│   │   └── site.yml            # Skupper site definition
│   └── iperf3-workload/        # Workload site resources
│       └── attached-connector.yml # Attached connector definition
├── README.md                   # This documentation
├── requirements.txt            # Python dependencies
└── test.yml                    # Main test playbook
```

## Requirements

- **Kubernetes**: One or more Kubernetes clusters with Skupper V2 installed
- **Ansible**: Core Ansible packages (listed in requirements.txt)
- **Collections**: Required Ansible collections (listed in collections/requirements.yml)
- **Kubeconfig**: Valid kubeconfig file with access to the target clusters

## Ansible Modules Used

This test primarily uses the following Ansible modules:

1. **kubernetes.core.k8s**: Creates, modifies, and deletes Kubernetes resources
2. **kubernetes.core.k8s_info**: Retrieves information about Kubernetes resources
3. **kubernetes.core.k8s_log**: Fetches logs from Kubernetes pods
4. **skupper.v2.resource**: Creates and manages Skupper resources
5. **skupper.v2.token**: Generates and manages Skupper access tokens
6. **ansible.builtin.assert**: Validates test conditions

## Skupper Resources Used

The test creates and uses the following Skupper resources:

1. **Site**: Skupper sites in each namespace
2. **Listener**: Service entry point in the client namespace
3. **AttachedConnector**: Links the iPerf3 server to the Skupper network
4. **AttachedConnectorBinding**: Binds the connector to the specific service

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

The test uses three logical hosts:
- `iperf3-hub`: Central Skupper site
- `iperf3-client`: Runs the iPerf3 client
- `iperf3-workload`: Runs the iPerf3 server

By default, all hosts use the same kubeconfig file. To test across different clusters, modify the kubeconfig paths in the corresponding host_vars files.

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

1. **Namespace Creation**: Creates namespaces for all sites with the prefix `e2e-`
2. **Workload Deployment**:
   - Deploys iPerf3 server in the workload namespace
   - Creates service for the iPerf3 server
3. **Skupper Resource Creation**:
   - Creates Skupper sites in all namespaces
   - Creates AttachedConnectorBinding in the hub namespace
   - Creates Listener in the client namespace
   - Creates AttachedConnector in the workload namespace
4. **Site Linking**:
   - Issues access token from the hub site
   - Applies token to the client site
5. **Validation**:
   - Waits for AttachedConnectorBinding to be ready
   - Creates an iPerf3 client job in the client namespace
   - Waits for the job to complete
   - Retrieves and validates job logs
6. **Cleanup**: Removes all created resources (unless skip_teardown is set)

## Key Configuration Parameters

### Hub Site
- **Namespace**: `e2e-iperf3-hub`
- **Resource**: AttachedConnectorBinding (links to workload namespace)

### Client Site
- **Namespace**: `e2e-iperf3-client`
- **Resource**: Listener (port 5201)
- **Test Job**: iPerf3 client connecting to the server

### Workload Site
- **Namespace**: `e2e-iperf3-workload`
- **Deployment**: iPerf3 server on port 5201
- **Resource**: AttachedConnector (links to hub namespace)

## Success Criteria

The test is considered successful when the iPerf3 client job completes and its logs contain:
- `"connected to"` message
- `"iperf Done."` message

This confirms that the network connection through the Skupper Attached Connector is working properly.

## Troubleshooting

### Common Issues

1. **Connection Failures**:
   - Verify kubeconfig is valid
   - Check if Skupper is properly installed
   - Ensure the iPerf3 server is running

2. **Resource Creation Errors**:
   - Verify RBAC permissions are sufficient
   - Check for namespace conflicts

3. **Attached Connector Not Ready**:
   - Check the status of the AttachedConnectorBinding
   - Verify the Skupper network is properly connected
   - Ensure the service selector matches the pod labels

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
   skupper status -n e2e-iperf3-hub
   skupper status -n e2e-iperf3-client
   skupper status -n e2e-iperf3-workload
   ```