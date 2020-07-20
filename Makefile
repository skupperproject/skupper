VERSION := $(shell git describe --tags --dirty=-modified --always)
SERVICE_CONTROLLER_IMAGE := quay.io/skupper/service-controller
SITE_CONTROLLER_IMAGE := quay.io/skupper/site-controller
TEST_IMAGE := quay.io/skupper/skupper-tests
TEST_BINARIES_FOLDER := ${PWD}/test/integration/bin
DOCKER := docker
LDFLAGS := -X github.com/skupperproject/skupper/client.Version=${VERSION}

all: build-cmd build-controllers build-tests

build-tests:
	mkdir -p ${TEST_BINARIES_FOLDER}
	go test -c -tags=integration -v ./test/integration/tcp_echo -o ${TEST_BINARIES_FOLDER}/tcp_echo_test
	go test -c -tags=integration -v ./test/integration/http -o ${TEST_BINARIES_FOLDER}/http_test
	go test -c -tags=integration -v ./test/integration/bookinfo -o ${TEST_BINARIES_FOLDER}/bookinfo_test
	go test -c -tags=integration -v ./test/integration/mongodb -o ${TEST_BINARIES_FOLDER}/mongo_test

build-cmd:
	go build -ldflags="${LDFLAGS}"  -o skupper cmd/skupper/skupper.go

build-service-controller:
	go build -ldflags="${LDFLAGS}"  -o service-controller cmd/service-controller/main.go cmd/service-controller/controller.go cmd/service-controller/service_sync.go cmd/service-controller/bridges.go cmd/service-controller/ports.go cmd/service-controller/definition_monitor.go cmd/service-controller/console_server.go cmd/service-controller/site_query.go cmd/service-controller/ip_lookup.go cmd/service-controller/config_sync.go

build-site-controller:
	go build -ldflags="${LDFLAGS}"  -o site-controller cmd/site-controller/main.go cmd/site-controller/controller.go

build-controllers: build-site-controller build-service-controller

build-service-controller-debug: BUILD_OPTS = -gcflags "all=-N -l"
build-service-controller-debug: build-service-controller

build-site-controller-debug: BUILD_OPTS = -gcflags "all=-N -l"
build-site-controller-debug: build-site-controller

build-controllers-debug: build-site-controller-debug build-service-controller-debug

docker-build-test-image:
	${DOCKER} build -t ${TEST_IMAGE} -f Dockerfile.ci-test .

docker-build: docker-build-test-image
	${DOCKER} build -t ${SERVICE_CONTROLLER_IMAGE}${DOCKER_SUFFIX} -f Dockerfile.service-controller${DOCKER_SUFFIX} .
	${DOCKER} build -t ${SITE_CONTROLLER_IMAGE}${DOCKER_SUFFIX} -f Dockerfile.site-controller${DOCKER_SUFFIX} .

docker-build-debug: DOCKER_SUFFIX = -debug
docker-build-debug: docker-build
	@echo
	@echo "Before running skupper init, make sure to export SKUPPER_SERVICE_CONTROLLER_IMAGE and SKUPPER_SITE_CONTROLLER_IMAGE"
	@echo "(pushing your debug images accordingly)"
	@echo
	@echo "You should also run: kubectl port-forward <podname> <local_port:remote_pod_port>"
	@echo
	@echo "Remote debug ports are:"
	@echo "service-controller: 40000"
	@echo "site-controller   : 40001"

docker-push-test-image:
	${DOCKER} push ${TEST_IMAGE}

docker-push: docker-push-test-image
	${DOCKER} push ${SERVICE_CONTROLLER_IMAGE}
	${DOCKER} push ${SITE_CONTROLLER_IMAGE}

format:
	go fmt ./...

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
	rm -rf skupper service-controller site-controller release ${TEST_BINARIES_FOLDER}

package: release/windows.zip release/darwin.zip release/linux.tgz

release/linux.tgz: release/linux/skupper
	tar -czf release/linux.tgz -C release/linux/ skupper

release/linux/skupper: cmd/skupper/skupper.go
	GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o release/linux/skupper cmd/skupper/skupper.go

release/windows/skupper: cmd/skupper/skupper.go
	GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o release/windows/skupper cmd/skupper/skupper.go

release/windows.zip: release/windows/skupper
	zip -j release/windows.zip release/windows/skupper

release/darwin/skupper: cmd/skupper/skupper.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o release/darwin/skupper cmd/skupper/skupper.go

release/darwin.zip: release/darwin/skupper
	zip -j release/darwin.zip release/darwin/skupper

