ROOT_PATH := $(shell pwd)
EXTRA_VARS := --extra-vars "@$(ROOT_PATH)/vars.yml"
COLLECTION_PATH := $(ROOT_PATH)/e2e/collections/ansible_collections/e2e/tests
TESTS_CI := attached-connector,ha,hello-world,expose-pods-by-name
PYTHON ?= python3

# The caller may set a TEST_PREFIX, so they can deal with all namespaces created
# by these tests with -l e2e.prefix=$TEST_PREFIX.  If not set, we create a random one
RANDOM != echo $$RANDOM
TEST_PREFIX ?= $(RANDOM)
export TEST_PREFIX

# E2E Test directories
E2E_TEST_DIRS := $(sort $(wildcard $(ROOT_PATH)/e2e/scenarios/*))

# Creating a python virtual environment
create-venv:
	@if [ "$(FORCE)" = "true" ] || [ ! -d "/tmp/e2e-venv" ]; then \
		bash -c "deactivate 2>/dev/null || true"; \
		echo "Removing old environment if it exists..."; \
		rm -rf /tmp/e2e-venv; \
		echo "Creating virtual environment..."; \
		cd /tmp && $(PYTHON) -m venv e2e-venv; \
		if [ -d "/tmp/e2e-venv" ]; then \
			echo "Virtual environment created successfully at /tmp/e2e-venv"; \
			. /tmp/e2e-venv/bin/activate && \
			pip3 install --upgrade pip; \
			pip3 install -r $(ROOT_PATH)/requirements.txt; \
			ANSIBLE_CONFIG=$(ROOT_PATH)/e2e/ansible.cfg ansible-galaxy collection install -r $(ROOT_PATH)/e2e/collections/ansible_collections/requirements.yml --force; \
		else \
			echo "Failed to create virtual environment at /tmp/e2e-venv"; \
			exit 1; \
		fi; \
	else \
		echo "Using the existing virtual environment at /tmp/e2e-venv"; \
		if [ ! -d "/tmp/e2e-venv" ]; then \
			echo "Virtual environment does not exist at /tmp/e2e-venv"; \
			echo "Run 'make create-venv FORCE=true' to create it"; \
			exit 1; \
		fi; \
	fi

# Run tests for a specific role given as parameter
test-role: create-venv
	# Returning if ROLE is not set
	@if [ -z "$(ROLE)" ]; then \
		echo "ROLE is not set. Please provide a ROLE to run the tests."; \
		exit 1; \
	fi
	@echo "Running the tests for the role: $(ROLE)"
	@cd $(COLLECTION_PATH) && \
	. /tmp/e2e-venv/bin/activate && \
	ANSIBLE_CONFIG=$(ROOT_PATH)/e2e/ansible.cfg ansible-playbook roles/$(ROLE)/tests/test_playbook.yml -i roles/$(ROLE)/tests/inventory $(EXTRA_VARS)

version: create-venv
	. /tmp/e2e-venv/bin/activate && \
	ANSIBLE_CONFIG=$(ROOT_PATH)/e2e/ansible.cfg \
	ansible-playbook --version

# Run a specific test (use relative path from e2e/ directory)
test: FORCE=false
test: create-venv
	@if [ -z "$(TEST)" ]; then \
		echo "TEST is not set. Please provide a TEST to run the tests."; \
		exit 1; \
	fi

	@echo "Running the test for directory: $(TEST) with prefix: $(TEST_PREFIX)";

	. /tmp/e2e-venv/bin/activate && \
	mkdir -p /tmp/e2e/$(TEST) && \
	ANSIBLE_LOG_PATH=/tmp/e2e/$(TEST)/ansible_$$(date +%Y%m%d_%H%M%S).log \
	ANSIBLE_CONFIG=$(ROOT_PATH)/e2e/ansible.cfg \
	ansible-playbook $(ROOT_PATH)/e2e/scenarios/$(TEST)/test.yml \
	-i $(ROOT_PATH)/e2e/scenarios/$(TEST)/inventory $(EXTRA_VARS) \
	$(OPTIONS) \
	-e namespace_prefix=$(TEST_PREFIX); \
	EXIT_CODE=$$?; \
	if [ $$EXIT_CODE -ne 0 ]; then \
		echo "Test $(TEST) failed with exit code $$EXIT_CODE"; \
	fi; \
	exit $$EXIT_CODE

# Run all tests in sequence
e2e-tests:
	@echo "Running all tests in sequence..."
	@for test_dir in $(E2E_TEST_DIRS); do \
		test_name=$$(basename $$test_dir); \
		echo "\n=== Running test: $$test_name ==="; \
		$(MAKE) test TEST="$$test_name" || exit 1; \
	done
	@echo -e "\n=== All tests completed successfully ==="

# Run a subset of tests in parallel (comma-separated list)
test-subset: FORCE=false
test-subset: create-venv
	@if [ -z "$(TESTS)" ]; then \
		echo "TESTS is not set. Please provide a comma-separated list of tests with TESTS=test1,test2,..."; \
		exit 1; \
	fi
	@echo "Running the following tests in parallel: $(TESTS)"
	@FAILED_TESTS=""; \
	for test in `echo $(TESTS) | tr ',' ' '`; do \
		$(MAKE) -j1 test TEST="$$test" & \
		TEST_PIDS="$$TEST_PIDS $$!"; \
	done; \
	for pid in $$TEST_PIDS; do \
		wait $$pid || FAILED_TESTS="$$FAILED_TESTS $$pid"; \
	done; \
	if [ -n "$$FAILED_TESTS" ]; then \
		echo "The following test processes failed: $$FAILED_TESTS"; \
		exit 1; \
	fi; \
	echo -e "\n=== All specified tests completed successfully ==="

# Run a subset of tests (comma-separated list) for CI
ci-tests: TESTS=$(TESTS_CI)
ci-tests: test-subset
