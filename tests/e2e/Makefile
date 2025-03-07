ROOT_PATH := $(shell pwd)
COLLECTION_PATH := $(ROOT_PATH)/collections/ansible_collections/e2e/tests
TAR_NAME := e2e-tests-0.1.11.tar.gz
TAR_PATH := $(COLLECTION_PATH)/$(TAR_NAME)

# Find all test directories that start with test_
TEST_DIRS := $(sort $(wildcard test_*))

build:
	@cd $(COLLECTION_PATH) && \
	ansible-galaxy collection build --force && \
	ansible-galaxy collection install -f $(TAR_PATH) --force && \
	cd $(ROOT_PATH)
	rm -rf $(COLLECTION_PATH)/$(TAR_NAME)
	
# Run tests for a specific role given as parameter
test-role:
	@cd $(COLLECTION_PATH) && \
	ansible-playbook roles/$(ROLE)/tests/test_playbook.yml -i roles/$(ROLE)/tests/inventory $(EXTRA_VARS)

test:
	ansible-playbook $(TEST)/test.yml -i $(TEST)/inventory $(EXTRA_VARS)

# Run all tests in directories starting with test_ in alphabetical order
all:
	@echo "Running all tests in sequence..."
	@for test_dir in $(TEST_DIRS); do \
		echo "\n=== Running test: $$test_dir ==="; \
		$(MAKE) test TEST=$$test_dir || exit 1; \
	done
	@echo "\n=== All tests completed successfully ==="
