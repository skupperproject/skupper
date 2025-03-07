ROOT_PATH := $(shell pwd)
EXTRA_VARS := --extra-vars "@$(ROOT_PATH)/vars.yml"

# Find all test directories that start with test_ in the e2e folder
E2E_TEST_DIRS := $(sort $(wildcard $(ROOT_PATH)/e2e/test_*))

# Creating a python virtual environment
# Creating a python virtual environment
create-venv:
	@if [ "$(FORCE)" = true ]; then \
		bash -c "deactivate 2>/dev/null || true"; \
		echo "Removing old environment if it exists..."; \
		rm -rf /tmp/e2e-venv; \
		echo "Creating virtual environment..."; \
		cd /tmp && python3 -m venv e2e-venv; \
		if [ -d "/tmp/e2e-venv" ]; then \
			echo "Virtual environment created successfully at /tmp/e2e-venv"; \
			bash -c "source /tmp/e2e-venv/bin/activate && \
			pip install --upgrade pip && \
			pip install -r $(ROOT_PATH)/requirements.txt"; \
			ansible-galaxy collection install -r $(ROOT_PATH)/e2e/collections/ansible_collections/requirements.yml; \
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
	@echo "Running the tests for the role: $(ROLE)"
	@cd $(COLLECTION_PATH) && \
	ansible-playbook roles/$(ROLE)/tests/test_playbook.yml -i roles/$(ROLE)/tests/inventory $(EXTRA_VARS)

# Run a specific test (use relative path from e2e/ directory)
test: FORCE=false
test: create-venv
	@echo "Running the test for directory: $(TEST)"
	ANSIBLE_CONFIG=$(ROOT_PATH)/e2e/ansible.cfg ansible-playbook $(ROOT_PATH)/e2e/$(TEST)/test.yml -i $(ROOT_PATH)/e2e/$(TEST)/inventory $(EXTRA_VARS)

# Run all tests in directories starting with test_ in alphabetical order
e2e-tests:
	@echo "Running all tests in sequence..."
	@for test_dir in $(E2E_TEST_DIRS); do \
		test_name=$$(basename $$test_dir); \
		echo "\n=== Running test: $$test_name ==="; \
		$(MAKE) test TEST="$$test_name" || exit 1; \
	done
	@echo "\n=== All tests completed successfully ==="
