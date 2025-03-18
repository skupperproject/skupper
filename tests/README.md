# Skupper Tests

This repository contains tests for Skupper, a layer 7 service interconnect solution that enables secure communication across Kubernetes clusters and other environments.

## Python Version

This repository requires Python 3.9 or later. Create a virtual environment and install the dependencies to run the tests. Please keep in mind that is your responsibility to ensure that the Python version is compatible with the dependencies.

```bash
# Under E2E test directory
python3.13 -m venv --upgrade-deps venv
source venv/bin/activate
pip install -r requirements.txt
```

Note: If you are running the tests from the Makefile, this is not needed, as the Makefile will create a virtual environment for you.

## Repository Structure

```
tests/
├── e2e/  
├── scenarios/                 # End-to-end tests directory
│    ├── hello-world/          # Basic Skupper functionality test
│    ├── iperf3-attached/      # Network performance test with attached connectors
│    ├── redis/                # Redis test
└── README.md                  # This file
```

## End-to-End (E2E) Tests

The `e2e` directory contains tests that validate Skupper functionality across different environments. Each test is organized into its own directory with all necessary files to run independently.

### Available E2E Tests

- **[hello-world](e2e/hello-world/)**: A simple test to verify basic Skupper functionality by deploying frontend and backend components across Skupper sites.
- **[iperf3-attached](e2e/iperf3-attached/)**: Tests the attached connector functionality by measuring network performance using iperf3.
- [redis](e2e/redis/): A test to validate Skupper connectivity using a Redis deployment.

## Test Requirements

To run the tests in this repository, you'll need:

1. **Skupper V2**: Installed on the target cluster(s)
2. **Kubernetes Access**: Valid kubeconfig with appropriate permissions
3. **Ansible**: Core Ansible packages and required collections
4. **Python**: 3.7+ with required dependencies

## Getting Started

Each test directory contains its own README with specific instructions, but here's the general process to run a E2E test:

### 1. Set Up Environment

```bash
# Create a Python virtual environment
make create-venv FORCE=true
```

> This will create a virtual environment at `/tmp/e2e-venv` and install all required dependencies (python and ansible). If the directory does not exist, it will be created and the virtual environment will be installed.

The Makefile will automatically:
- Create a Python virtual environment if needed
- Install all required dependencies
- Install necessary Ansible collections
- Run the test with the proper configuration
- When you trigger a test it will create a namespace_prefix to avoid conflicts with other tests


### 2. Run Test

```bash
# Run a specific test
make test TEST="hello-world"
```

- This will run the `hello-world` test located in the `e2e/hello-world` directory, activating the virtual environment and running the test playbook.

### Aditional Configuration

1. **vars.yml file**: Create a `vars.yml` file in the repository root to set extra variables for the tests.

2. **Available Make Commands**:

```bash
# Create or refresh the virtual environment
make create-venv FORCE=true

# Testing a specific role
make test-role ROLE="role_name"

# Run a specific test
make test TEST="test_directory_name"

# Run all tests (all directories in e2e/ that start with test_)
make e2e-tests
```

### Example summary

```bash
# Create a new virtual environment
make create-venv FORCE=true

# Run a specific test
make test TEST="hello_world"

# Run all tests in sequence
make e2e-tests
```

## Core Ansible Collections

The tests rely on the following Ansible collections:

- **ansible.posix
- **ansible.scm**
- **ansible.utils**
- **kubernetes.core**
- **skupper.v2
- **e2e.tests
- containers.podman

## Contributing

When adding new tests to this repository, please follow these guidelines:

1. **Directory Structure**: Create a new directory under `e2e/` with a descriptive name
2. **Documentation**: Include a comprehensive README.md
3. **Requirements**: List all dependencies in requirements.txt
4. **Testing**: Ensure tests have proper validation and cleanup

A standard test directory should include:

```
e2e/scenarios/your-test/
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
