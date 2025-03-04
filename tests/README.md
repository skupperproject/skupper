# Skupper Tests

This repository contains tests for Skupper, a layer 7 service interconnect solution that enables secure communication across Kubernetes clusters and other environments.

## Repository Structure

```
tests/
├── e2e/                      # End-to-end tests directory
│   ├── hello-world/          # Basic Skupper functionality test
│   └── iperf3-attached/      # Network performance test with attached connectors
└── README.md                 # This file
```

## End-to-End (E2E) Tests

The `e2e` directory contains tests that validate Skupper functionality across different environments. Each test is organized into its own directory with all necessary files to run independently.

### Available E2E Tests

- **[hello-world](e2e/hello-world/)**: A simple test to verify basic Skupper functionality by deploying frontend and backend components across Skupper sites.
- **[iperf3-attached](e2e/iperf3-attached/)**: Tests the attached connector functionality by measuring network performance using iperf3.

## Test Requirements

To run the tests in this repository, you'll need:

1. **Skupper V2**: Installed on the target cluster(s)
2. **Kubernetes Access**: Valid kubeconfig with appropriate permissions
3. **Ansible**: Core Ansible packages and required collections
4. **Python**: 3.7+ with required dependencies

## Getting Started

Each test directory contains its own README with specific instructions, but here's the general process:

### 1. Set Up Environment

```bash
# Create a virtual environment
python3 -m venv .venv

# Activate the virtual environment
source .venv/bin/activate  # On Linux/Mac
# OR
.venv\Scripts\activate     # On Windows
```

### 2. Install Dependencies

```bash
# Navigate to the specific test directory
cd e2e/hello-world/  # Example

# Install Python dependencies
pip install -r requirements.txt

# Install Ansible collections
ansible-galaxy collection install -r collections/requirements.yml
```

### 3. Run Test

```bash
# Run the test playbook
ansible-playbook test.yml -i inventory
```

## Core Ansible Collections

The tests rely on the following Ansible collections:

- **ansible.posix** (v1.4.0)
- **ansible.scm** (v2.0.0)
- **ansible.utils** (v4.0.0)
- **kubernetes.core** (v3.2.0)
- **skupper.v2** (v2.0.0-preview-1)
- **rhsiqe.skupper** (from GitHub: rafaelvzago/skupper-tests)

## Contributing

When adding new tests to this repository, please follow these guidelines:

1. **Directory Structure**: Create a new directory under `e2e/` with a descriptive name
2. **Documentation**: Include a comprehensive README.md
3. **Requirements**: List all dependencies in requirements.txt
4. **Testing**: Ensure tests have proper validation and cleanup

A standard test directory should include:

```
e2e/your-test/
├── ansible.cfg           # Ansible configuration
├── collections/          # Ansible collections
│   └── requirements.yml  # Collection dependencies
├── inventory/            # Test inventory
├── README.md             # Test documentation
├── requirements.txt      # Python dependencies
└── test.yml              # Main test playbook
```

## License

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](../LICENSE) file for more details.

## Contact

For questions or issues, please open an issue in this repository.