VERSION := $(shell git describe --tags --dirty=-modified --always)
SERVICE_CONTROLLER_IMAGE := quay.io/skupper/service-controller
SITE_CONTROLLER_IMAGE := quay.io/skupper/site-controller
TEST_IMAGE := quay.io/skupper/skupper-tests
TEST_BINARIES_FOLDER := ${PWD}/test/integration/bin
DOCKER := docker


all: build-cmd build-controllers build-tests

build-tests:
	mkdir -p ${TEST_BINARIES_FOLDER}
	go test -c -tags=integration -v ./test/integration/tcp_echo -o ${TEST_BINARIES_FOLDER}/tcp_echo_test
	go test -c -tags=integration -v ./test/integration/http -o ${TEST_BINARIES_FOLDER}/http_test
	go test -c -tags=integration -v ./test/integration/bookinfo -o ${TEST_BINARIES_FOLDER}/bookinfo_test

build-cmd:
	go build -ldflags="-X main.version=${VERSION}"  -o skupper cmd/skupper/skupper.go

build-service-controller:
	go build -ldflags="-X main.version=${VERSION}"  -o service-controller cmd/service-controller/main.go cmd/service-controller/controller.go cmd/service-controller/service_sync.go cmd/service-controller/bridges.go cmd/service-controller/ports.go cmd/service-controller/definition_monitor.go cmd/service-controller/console_server.go cmd/service-controller/site_query.go cmd/service-controller/ip_lookup.go

build-site-controller:
	go build -ldflags="-X main.version=${VERSION}"  -o site-controller cmd/site-controller/main.go cmd/site-controller/controller.go

build-controllers: build-site-controller build-service-controller

docker-build-test-image:
	${DOCKER} build -t ${TEST_IMAGE} -f Dockerfile.ci-test .

docker-build: docker-build-test-image
	${DOCKER} build -t ${SERVICE_CONTROLLER_IMAGE} -f Dockerfile.service-controller .
	${DOCKER} build -t ${SITE_CONTROLLER_IMAGE} -f Dockerfile.site-controller .

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
	GOOS=linux GOARCH=amd64 go build -ldflags="-X main.version=${VERSION}" -o release/linux/skupper cmd/skupper/skupper.go

release/windows/skupper: cmd/skupper/skupper.go
	GOOS=windows GOARCH=amd64 go build -ldflags="-X main.version=${VERSION}" -o release/windows/skupper cmd/skupper/skupper.go

release/windows.zip: release/windows/skupper
	zip -j release/windows.zip release/windows/skupper

release/darwin/skupper: cmd/skupper/skupper.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.version=${VERSION}" -o release/darwin/skupper cmd/skupper/skupper.go

release/darwin.zip: release/darwin/skupper
	zip -j release/darwin.zip release/darwin/skupper

