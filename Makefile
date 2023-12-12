VERSION := $(shell git describe --tags --dirty=-modified --always)
SERVICE_CONTROLLER_IMAGE := quay.io/skupper/service-controller
CONTROLLER_PODMAN_IMAGE := quay.io/skupper/controller-podman
SITE_CONTROLLER_IMAGE := quay.io/skupper/site-controller
CONFIG_SYNC_IMAGE := quay.io/skupper/config-sync
FLOW_COLLECTOR_IMAGE := quay.io/skupper/flow-collector
TEST_IMAGE := quay.io/skupper/skupper-tests
TEST_BINARIES_FOLDER := ${PWD}/test/integration/bin
DOCKER := docker
LDFLAGS := -X github.com/skupperproject/skupper/pkg/version.Version=${VERSION}
PLATFORMS ?= linux/amd64,linux/arm64
GOOS ?= linux
GOARCH ?= amd64

# Shipyard configuration
BASE_BRANCH = main
LOAD_BALANCER = true
ORG = skupperproject
PROJECT = skupper
SETTINGS = ./.shipyard.yml
SHIPYARD_URL = https://raw.githubusercontent.com/submariner-io/shipyard/devel
export BASE_BRANCH ORG PROJECT SHIPYARD_REPO SHIPYARD_URL

all: generate-client build-cmd build-get build-config-sync build-controllers build-tests build-manifest

build-tests:
	mkdir -p ${TEST_BINARIES_FOLDER}
	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/tcp_echo/job -o ${TEST_BINARIES_FOLDER}/tcp_echo_test
	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/http/job -o ${TEST_BINARIES_FOLDER}/http_test
	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/bookinfo/job -o ${TEST_BINARIES_FOLDER}/bookinfo_test
	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/mongodb/job -o ${TEST_BINARIES_FOLDER}/mongo_test
	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/custom/hipstershop/job -o ${TEST_BINARIES_FOLDER}/grpcclient_test
	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/tls_t/job -o ${TEST_BINARIES_FOLDER}/tls_test

build-cmd: generate-client
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o skupper ./cmd/skupper

build-get:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o get ./cmd/get

build-service-controller:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o service-controller cmd/service-controller/main.go cmd/service-controller/controller.go cmd/service-controller/ports.go cmd/service-controller/definition_monitor.go cmd/service-controller/console_server.go cmd/service-controller/site_query.go cmd/service-controller/ip_lookup.go cmd/service-controller/token_handler.go cmd/service-controller/secret_controller.go cmd/service-controller/claim_handler.go cmd/service-controller/tokens.go cmd/service-controller/links.go cmd/service-controller/services.go cmd/service-controller/policies.go cmd/service-controller/policy_controller.go cmd/service-controller/revoke_access.go  cmd/service-controller/nodes.go

build-controller-podman:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o controller-podman cmd/controller-podman/main.go

build-site-controller:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o site-controller cmd/site-controller/main.go cmd/site-controller/controller.go

build-flow-collector:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o flow-collector cmd/flow-collector/main.go cmd/flow-collector/controller.go cmd/flow-collector/handlers.go

build-config-sync:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o config-sync cmd/config-sync/main.go cmd/config-sync/config_sync.go cmd/config-sync/collector.go

build-controllers: build-site-controller build-service-controller build-controller-podman build-flow-collector

build-manifest:
	go build -ldflags="${LDFLAGS}"  -o manifest ./cmd/manifest

docker-build-test-image:
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${TEST_IMAGE} -f Dockerfile.ci-test .
	${DOCKER} buildx build --load -t ${TEST_IMAGE} -f Dockerfile.ci-test .

docker-build: generate-client docker-build-test-image
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${SERVICE_CONTROLLER_IMAGE} -f Dockerfile.service-controller .
	${DOCKER} buildx build --load  -t ${SERVICE_CONTROLLER_IMAGE} -f Dockerfile.service-controller .
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${CONTROLLER_PODMAN_IMAGE} -f Dockerfile.controller-podman .
	${DOCKER} buildx build --load  -t ${CONTROLLER_PODMAN_IMAGE} -f Dockerfile.controller-podman .
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${SITE_CONTROLLER_IMAGE} -f Dockerfile.site-controller .
	${DOCKER} buildx build --load  -t ${SITE_CONTROLLER_IMAGE} -f Dockerfile.site-controller .
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${CONFIG_SYNC_IMAGE} -f Dockerfile.config-sync .
	${DOCKER} buildx build --load  -t ${CONFIG_SYNC_IMAGE} -f Dockerfile.config-sync .
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${FLOW_COLLECTOR_IMAGE} -f Dockerfile.flow-collector .
	${DOCKER} buildx build --load  -t ${FLOW_COLLECTOR_IMAGE} -f Dockerfile.flow-collector .

docker-push-test-image:
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${TEST_IMAGE} -f Dockerfile.ci-test .

docker-push: docker-push-test-image
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${SERVICE_CONTROLLER_IMAGE} -f Dockerfile.service-controller .
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${CONTROLLER_PODMAN_IMAGE} -f Dockerfile.controller-podman .
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${SITE_CONTROLLER_IMAGE} -f Dockerfile.site-controller .
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${CONFIG_SYNC_IMAGE} -f Dockerfile.config-sync .
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${FLOW_COLLECTOR_IMAGE} -f Dockerfile.flow-collector .

format:
	go fmt ./...

generate-client:
	./scripts/update-codegen.sh
	./scripts/libpod-generate.sh

force-generate-client:
	FORCE=true ./scripts/update-codegen.sh
	FORCE=true ./scripts/libpod-generate.sh

client-mock-test:
	go test -v -count=1 ./client

client-cluster-test:
	go test -v -count=1 ./client -use-cluster

vet:
	go vet ./...

cmd-test:
	go test -v -count=1 ./cmd/...

pkg-test:
	go test -v -count=1 ./pkg/...

.PHONY: test
test:
	go test -v -count=1 ./pkg/... ./cmd/... ./client/...

clean:
	rm -rf skupper service-controller controller-podman site-controller release get config-sync manifest ${TEST_BINARIES_FOLDER}

package: release/windows.zip release/darwin.zip release/linux.tgz release/s390x.tgz release/arm64.tgz

release/linux.tgz: release/linux/skupper
	tar -czf release/linux.tgz -C release/linux/ skupper

release/linux/skupper: cmd/skupper/skupper.go
	GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o release/linux/skupper ./cmd/skupper

release/windows/skupper: cmd/skupper/skupper.go
	GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o release/windows/skupper ./cmd/skupper

release/windows.zip: release/windows/skupper
	zip -j release/windows.zip release/windows/skupper

release/darwin/skupper: cmd/skupper/skupper.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o release/darwin/skupper ./cmd/skupper

release/darwin.zip: release/darwin/skupper
	zip -j release/darwin.zip release/darwin/skupper

generate-manifest: build-manifest
	./manifest

release/s390x/skupper: cmd/skupper/skupper.go
	GOOS=linux GOARCH=s390x go build -ldflags="${LDFLAGS}" -o release/s390x/skupper ./cmd/skupper

release/s390x.tgz: release/s390x/skupper
	tar -czf release/s390x.tgz release/s390x/skupper

release/arm64/skupper: cmd/skupper/skupper.go
	GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o release/arm64/skupper ./cmd/skupper

release/arm64.tgz: release/arm64/skupper
	tar -czf release/arm64.tgz release/arm64/skupper

ifneq (,$(DAPPER_HOST_ARCH))

# Running in Shipyard's container

include $(SHIPYARD_DIR)/Makefile.clusters

else

# Not running in Shipyard's container

Makefile.shipyard:
ifeq (,$(findstring s,$(firstword -$(MAKEFLAGS))))
	@echo Downloading $@
endif
	@curl -sfLO $(SHIPYARD_URL)/$@

ONLY_SHIPYARD_GOALS = true
include Makefile.shipyard

endif
