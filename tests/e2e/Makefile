ROOT_PATH := $(shell pwd)
COLLECTION_PATH := $(ROOT_PATH)/collections/ansible_collections/e2e/tests
TAR_NAME := e2e-tests-0.1.11.tar.gz
TAR_PATH := $(COLLECTION_PATH)/$(TAR_NAME)

build:
	@cd $(COLLECTION_PATH) && \
	ansible-galaxy collection build --force && \
	ansible-galaxy collection install -f $(TAR_PATH) --force && \
	cd $(ROOT_PATH)
	rm -rf $(COLLECTION_PATH)/$(TAR_NAME)
	
# Run tests for a specific role given as parameter
test:
	@cd $(COLLECTION_PATH) && \
	ansible-playbook roles/$(ROLE)/tests/test_playbook.yml -i roles/$(ROLE)/tests/inventory -v $(EXTRA_VARS)