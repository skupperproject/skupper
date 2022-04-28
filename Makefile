VERSION := $(shell git describe --tags --dirty=-modified --always)
SERVICE_CONTROLLER_IMAGE := quay.io/skupper/service-controller
SITE_CONTROLLER_IMAGE := quay.io/skupper/site-controller
CONFIG_SYNC_IMAGE := quay.io/skupper/config-sync
TEST_IMAGE := quay.io/skupper/skupper-tests
TEST_BINARIES_FOLDER := ${PWD}/test/integration/bin
DOCKER := docker
LDFLAGS := -X github.com/skupperproject/skupper/client.Version=${VERSION}

all: build-cmd build-get build-config-sync build-controllers build-tests

build-tests:
	mkdir -p ${TEST_BINARIES_FOLDER}
	go test -c -tags=job -v ./test/integration/examples/tcp_echo/job -o ${TEST_BINARIES_FOLDER}/tcp_echo_test
	go test -c -tags=job -v ./test/integration/examples/http/job -o ${TEST_BINARIES_FOLDER}/http_test
	go test -c -tags=job -v ./test/integration/examples/bookinfo/job -o ${TEST_BINARIES_FOLDER}/bookinfo_test
	go test -c -tags=job -v ./test/integration/examples/mongodb/job -o ${TEST_BINARIES_FOLDER}/mongo_test
	go test -c -tags=job -v ./test/integration/examples/custom/hipstershop/job -o ${TEST_BINARIES_FOLDER}/grpcclient_test

build-cmd:
	go build -ldflags="${LDFLAGS}"  -o skupper ./cmd/skupper

build-get:
	go build -ldflags="${LDFLAGS}"  -o get ./cmd/get

build-service-controller:
	go build -ldflags="${LDFLAGS}"  -o service-controller cmd/service-controller/main.go cmd/service-controller/controller.go cmd/service-controller/bridges.go cmd/service-controller/ports.go cmd/service-controller/definition_monitor.go cmd/service-controller/console_server.go cmd/service-controller/site_query.go cmd/service-controller/ip_lookup.go cmd/service-controller/claim_verifier.go cmd/service-controller/token_handler.go cmd/service-controller/secret_controller.go cmd/service-controller/claim_handler.go cmd/service-controller/tokens.go cmd/service-controller/links.go cmd/service-controller/services.go cmd/service-controller/policies.go cmd/service-controller/policy_controller.go

build-site-controller:
	go build -ldflags="${LDFLAGS}"  -o site-controller cmd/site-controller/main.go cmd/site-controller/controller.go

build-config-sync:
	go build -ldflags="${LDFLAGS}"  -o config-sync cmd/config-sync/main.go cmd/config-sync/config_sync.go

build-controllers: build-site-controller build-service-controller

docker-build-test-image:
	${DOCKER} build -t ${TEST_IMAGE} -f Dockerfile.ci-test .

docker-build: docker-build-test-image
	${DOCKER} build -t ${SERVICE_CONTROLLER_IMAGE} -f Dockerfile.service-controller .
	${DOCKER} build -t ${SITE_CONTROLLER_IMAGE} -f Dockerfile.site-controller .
	${DOCKER} build -t ${CONFIG_SYNC_IMAGE} -f Dockerfile.config-sync .

docker-push-test-image:
	${DOCKER} push ${TEST_IMAGE}

docker-push: docker-push-test-image
	${DOCKER} push ${SERVICE_CONTROLLER_IMAGE}
	${DOCKER} push ${SITE_CONTROLLER_IMAGE}
	${DOCKER} push ${CONFIG_SYNC_IMAGE}

format:
	go fmt ./...

generate-client:
	./scripts/update-codegen.sh

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
	rm -rf skupper service-controller site-controller release get config-sync ${TEST_BINARIES_FOLDER}

package: release/windows.zip release/darwin.zip release/linux.tgz

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

