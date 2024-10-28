VERSION := $(shell git describe --tags --dirty=-modified --always)
CONTROLLER_IMAGE := quay.io/skupper/controller:v2-latest
BOOTSTRAP_IMAGE := quay.io/skupper/bootstrap:v2-latest
ADAPTOR_IMAGE := quay.io/skupper/kube-adaptor:v2-latest
NETWORK_CONSOLE_COLLECTOR_IMAGE := quay.io/skupper/network-console-collector:v2-latest
TEST_IMAGE := quay.io/skupper/skupper-tests:v2-latest
TEST_BINARIES_FOLDER := ${PWD}/test/integration/bin
DOCKER := docker
LDFLAGS := -X github.com/skupperproject/skupper/pkg/version.Version=${VERSION}
TESTFLAGS := -v -race -short
PLATFORMS ?= linux/amd64,linux/arm64
GOOS ?= linux
GOARCH ?= amd64

all: build-cmd build-kube-adaptor build-controller build-bootstrap build-tests build-manifest build-network-console-collector update-helm-crd

build-tests:
	mkdir -p ${TEST_BINARIES_FOLDER}
#	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/tcp_echo/job -o ${TEST_BINARIES_FOLDER}/tcp_echo_test
#	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/http/job -o ${TEST_BINARIES_FOLDER}/http_test
#	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/bookinfo/job -o ${TEST_BINARIES_FOLDER}/bookinfo_test
#	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/mongodb/job -o ${TEST_BINARIES_FOLDER}/mongo_test
#	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/custom/hipstershop/job -o ${TEST_BINARIES_FOLDER}/grpcclient_test
#	GOOS=${GOOS} GOARCH=${GOARCH} go test -c -tags=job -v ./test/integration/examples/tls_t/job -o ${TEST_BINARIES_FOLDER}/tls_test

build-cmd:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o skupper ./cmd/skupper

build-bootstrap:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o bootstrap ./cmd/bootstrap

build-controller:
	go build -ldflags="${LDFLAGS}"  -o controller cmd/controller/main.go cmd/controller/controller.go

build-kube-adaptor:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o kube-adaptor cmd/kube-adaptor/main.go

build-network-console-collector:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o network-console-collector ./cmd/network-console-collector

build-manifest:
	go build -ldflags="${LDFLAGS}"  -o manifest ./cmd/manifest

build-doc-generator:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o generate-doc ./internal/cmd/generate-doc

docker-build-test-image:
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${TEST_IMAGE} -f Dockerfile.ci-test .
	${DOCKER} buildx build --load -t ${TEST_IMAGE} -f Dockerfile.ci-test .

docker-build: docker-build-test-image docker-build-bootstrap docker-build-network-console-collector
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${CONTROLLER_IMAGE} -f Dockerfile.controller .
	${DOCKER} buildx build --load  -t ${CONTROLLER_IMAGE} -f Dockerfile.controller .
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${ADAPTOR_IMAGE} -f Dockerfile.kube-adaptor .
	${DOCKER} buildx build --load  -t ${ADAPTOR_IMAGE} -f Dockerfile.kube-adaptor .

docker-build-bootstrap:
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${BOOTSTRAP_IMAGE} -f Dockerfile.bootstrap .
	${DOCKER} buildx build --load  -t ${BOOTSTRAP_IMAGE} -f Dockerfile.bootstrap .

docker-push-bootstrap:
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${BOOTSTRAP_IMAGE} -f Dockerfile.bootstrap .

docker-build-network-console-collector:
	${DOCKER} buildx build --platform ${PLATFORMS} -t ${NETWORK_CONSOLE_COLLECTOR_IMAGE} -f Dockerfile.network-console-collector .
	${DOCKER} buildx build --load  -t ${NETWORK_CONSOLE_COLLECTOR_IMAGE} -f Dockerfile.network-console-collector .

docker-push-network-console-collector:
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${NETWORK_CONSOLE_COLLECTOR_IMAGE} -f Dockerfile.network-console-collector .

docker-push-test-image:
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${TEST_IMAGE} -f Dockerfile.ci-test .

docker-push: docker-push-test-image docker-push-bootstrap docker-push-network-console-collector
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${ADAPTOR_IMAGE} -f Dockerfile.kube-adaptor .
	${DOCKER} buildx build --push --platform ${PLATFORMS} -t ${CONTROLLER_IMAGE} -f Dockerfile.controller .

format:
	go fmt ./...

generate-client:
	./scripts/update-codegen.sh

force-generate-client:
	FORCE=true ./scripts/update-codegen.sh

vet:
	go vet ./...

.PHONY: test
test:
	go test ${TESTFLAGS} ./...

.PHONY: cover
cover:
	go test ${TESTFLAGS} \
		-cover \
		-coverprofile cover.out \
		./...

clean:
	rm -rf skupper controller release kube-adaptor manifest bootstrap network-console-collector ${TEST_BINARIES_FOLDER}

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

generate-doc: build-doc-generator
	./generate-doc ./doc/cli

release/s390x/skupper: cmd/skupper/skupper.go
	GOOS=linux GOARCH=s390x go build -ldflags="${LDFLAGS}" -o release/s390x/skupper ./cmd/skupper

release/s390x.tgz: release/s390x/skupper
	tar -czf release/s390x.tgz release/s390x/skupper

release/arm64/skupper: cmd/skupper/skupper.go
	GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o release/arm64/skupper ./cmd/skupper

release/arm64.tgz: release/arm64/skupper
	tar -czf release/arm64.tgz release/arm64/skupper

update-helm-crd:
	./scripts/update-helm-crds.sh
