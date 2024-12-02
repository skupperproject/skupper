VERSION := $(shell git describe --tags --dirty=-modified --always)
REVISION := $(shell git rev-parse HEAD)

LDFLAGS_EXTRA ?= -s -w # default to building stripped executables
LDFLAGS := ${LDFLAGS_EXTRA} -X github.com/skupperproject/skupper/pkg/version.Version=${VERSION}
TESTFLAGS := -v -race -short
GOOS ?= linux
GOARCH ?= amd64

REGISTRY := quay.io/skupper
IMAGE_TAG := v2-latest
PLATFORMS ?= linux/amd64,linux/arm64
CONTAINERFILES := Dockerfile.bootstrap Dockerfile.config-sync Dockerfile.controller Dockerfile.network-observer
SHARED_IMAGE_LABELS = \
    --label "org.opencontainers.image.created=$(shell TZ=GMT date --iso-8601=seconds)" \
	--label "org.opencontainers.image.url=https://skupper.io/" \
	--label "org.opencontainers.image.documentation=https://skupper.io/" \
	--label "org.opencontainers.image.source=https://github.com/skupperproject/skupper" \
	--label "org.opencontainers.image.version=${VERSION}" \
	--label "org.opencontainers.image.revision=${REVISION}" \
	--label "org.opencontainers.image.licenses=Apache-2.0"


DOCKER := docker
SKOPEO := skopeo
PODMAN := podman

all: build-cmd build-config-sync build-controller build-bootstrap build-network-observer update-helm-crd

build-cmd:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o skupper ./cmd/skupper

build-bootstrap:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o bootstrap ./cmd/bootstrap

build-controller:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o controller ./cmd/controller

build-config-sync:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o config-sync ./cmd/config-sync

build-network-observer:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o network-observer ./cmd/network-observer

build-doc-generator:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o generate-doc ./internal/cmd/generate-doc

## native/default container image builds
docker-build: $(patsubst Dockerfile.%,docker-build-%,$(CONTAINERFILES))
docker-build-%: Dockerfile.%
	${DOCKER} build $(SHARED_IMAGE_LABELS) -t "${REGISTRY}/$*:${IMAGE_TAG}" -f $< .

podman-build: $(patsubst Dockerfile.%,podman-build-%,$(CONTAINERFILES))
podman-build-%: Dockerfile.%
	${PODMAN} build $(SHARED_IMAGE_LABELS) -t "${REGISTRY}/$*:${IMAGE_TAG}" -f $< .


## multi-platform container images built in docker buildkit builder and
# exported to oci archive format.
multiarch-oci: $(patsubst Dockerfile.%,multiarch-oci-%,$(CONTAINERFILES))
multiarch-oci-%: Dockerfile.% oci-archives
	${DOCKER} buildx build \
		"--output=type=oci,dest=$(shell pwd)/oci-archives/$*.tar" \
		-t "${REGISTRY}/$*:${IMAGE_TAG}" \
		$(SHARED_IMAGE_LABELS) \
		--platform ${PLATFORMS} \
		-f $< .

## push multiarch-oci images to a registry using skopeo
push-multiarch-oci: $(patsubst Dockerfile.%,push-multiarch-oci-%,$(CONTAINERFILES))
push-multiarch-oci-%: ./oci-archives/%.tar
	${SKOPEO} copy --all \
		oci-archive:$< \
		"docker://${REGISTRY}/$*:${IMAGE_TAG}"

## Load images from oci-archive into local image storage
podman-load-oci:
	for archive in ./oci-archives/*.tar; do ${PODMAN} load < "$$archive"; done
## Has unfortunate podman dependency; docker image load does not load OCI archives, while podman does.
docker-load-oci:
	for archive in ./oci-archives/*.tar; do \
		img=$$(${PODMAN} load -q < "$$archive" | awk -F": " '{print $$2}') \
		&& ${PODMAN} image save "$$img" | ${DOCKER} load; \
	done

## Print fully qualified image names by arch
describe-multiarch-oci:
	@scripts/oci-index-archive-info.sh amd64 arm64

oci-archives:
	mkdir -p oci-archives

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

generate-manifest: build-cmd
	./skupper version -o json > manifest.json

generate-doc: build-doc-generator
	./generate-doc ./doc/cli

update-helm-crd:
	./scripts/update-helm-crds.sh

clean:
	rm -rf skupper controller config-sync \
		bootstrap network-observer generate-doc \
		cover.out oci-archives
