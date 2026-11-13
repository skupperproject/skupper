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
│    ├── attached-connector/   # Network performance test with attached connectors
│    ├── redis/                # Redis test
│    ├── ha/                   # High availability test using Skupper
└── README.md                  # This file
```

## End-to-End (E2E) Tests

The `e2e` directory contains tests that validate Skupper functionality across different environments. Each test is organized into its own directory with all necessary files to run independently.

### Available E2E Tests

- **[hello-world](e2e/scenarios/hello-world/)**: A simple test to verify basic Skupper functionality by deploying frontend and backend components across Skupper sites.
- **[attached-connector](e2e/scenarios/attached-connector/)**: A test to validate Skupper connectivity using attached connectors, including network performance testing with iperf3.
- **[redis](e2e/scenarios/redis/)**: A test to validate Skupper functionality with Redis, including data persistence and replication.
- **[ha](e2e/scenarios/ha/)**: A test to validate Skupper's high availability mode.

## Adding a new E2E test to be run on CI

To add a new E2E test to be run on CI, follow these steps:

1. Create your test in the `e2e/scenarios/` directory.
2. Edit the `Makefile` in the tests directory to include your test in the `ci-tests` target.
```bash
# Run a subset of tests (comma-separated list) for CI
ci-tests: TESTS=hello-world,attached-connector,YOUR_TEST
```
3. Ensure your test has a README.md file with instructions on how to run it.

## Resource Multipliers

In the `vars.yml` file, you can configure the multipliers for resource retries and delays using the following variables:

- `RESOURCE_RETRY_MULTIPLIER`: Multiplies the base retry value for resource operations.
- `RESOURCE_DELAY_MULTIPLIER`: Multiplies the base delay value between retries.

These multipliers allow you to adjust the retry and delay behavior dynamically based on your testing needs.

### Usage in Tests

In the `test.yml` file, these multipliers are used to control the retry and delay logic for Kubernetes operations. For example:

```yaml
retries: "{{ resource_retry_value * RESOURCE_RETRY_MULTIPLIER }}"
delay: "{{ resource_delay_value * RESOURCE_DELAY_MULTIPLIER }}"
```

This configuration helps manage network latency and resource availability issues during test execution.

Example usage in `vars.yml`:
```yaml
RESOURCE_RETRY_MULTIPLIER: 2
RESOURCE_DELAY_MULTIPLIER: 3
```

This configuration will double the retry attempts and triple the delay duration for resource operations.

## Test Requirements

To run the tests in this repository, you'll need:

1. **Skupper V2**: Installed on the target cluster(s)
2. **Kubernetes Access**: Valid kubeconfig with appropriate permissions
3. **Ansible**: Core Ansible packages and required collections
4. **Python**: 3.9+ with required dependencies

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

**Prerequisites**

* A cluster with Skupper installed cluster-wide
* Your kubeconfig is set to that cluster

If you are testing changes to Skupper code, create the images you want to test by running the following command in the project root folder:

```
# Under the project root folder
make podman-build
```

**Tip:** To quickly create a [kind](https://kind.sigs.k8s.io/) cluster:

```bash
KUBECONFIG=~/.kube/config ../scripts/kind-dev-cluster -r --metallb -i podman
```

**Note:** The scripts create and delete namespaces.

```bash
# Run a specific test
make test TEST="hello-world"
```

- This will run the `hello-world` test located in the `e2e/hello-world` directory, activating the virtual environment and running the test playbook.

### Additional Configuration

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

# Run a specific test with a subset of tests
make test-subset TESTS="test1,test2"

# Run CI tests
make ci-tests
```

3. **Make variables**

The following `make` variables are available:

* `OPTIONS` will provide additional options to the `ansible-playbook` invocation.

  `make e2e-tests OPTIONS="-v"`

* `PYTHON` allows the `venv` to be created with a specific Python binary

  `make create-venv PYTHON=python3.12`

* `TEST_PREFIX` allows one to select a prefix to be added to the name of the tests' namespaces:

  `make e2e-tests TEST_PREFIX=123`

  By default, a random prefix is generated for each run.  The namespaces can be selected on Kubernetes
  with the label `e2e.prefix`, allowing for easy removal of failed tests, for example.

### Example summary

```bash
# Create a new virtual environment
make create-venv FORCE=true

# Run a specific test
make test TEST="hello_world"

# Run all tests in sequence
make e2e-tests

# Running a subset of tests
make test-subset TESTS=hello-world,attached-connector

# Running CI tests
make ci-tests
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

1. **Directory Structure**: Create a new directory under `e2e/scenario` with a descriptive name
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

## Creating a New E2E Test

To create a new end-to-end (E2E) test, it is recommended to use the `hello-world` test as a base. The `hello-world` test is the simplest and most basic test, making it an ideal starting point for new tests. Follow these steps:

1. **Copy the Base Test**: Duplicate the `hello-world` directory located in `e2e/scenarios/`.

   ```bash
   cp -r tests/e2e/scenarios/hello-world tests/e2e/scenarios/your-new-test
   ```

2. **Modify the Test Name**: Rename the copied directory and update any references to `hello-world` within the test files to reflect the new test name.

3. **Customize the Test**: Adjust the test logic, configuration, and any other necessary components to fit the new test scenario.

4. **Document the Test**: Ensure that the new test directory includes a `README.md` file with instructions on how to run the test.

By following these steps, you can efficiently create a new E2E test that integrates seamlessly with the existing test framework.

## License

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](../LICENSE) file for more details.

## Contact

For questions or issues, please open an issue in this repository.
