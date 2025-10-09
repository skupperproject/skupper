VERSION := $(shell git describe --tags --dirty=-modified --always)
REVISION := $(shell git rev-parse HEAD)

LDFLAGS_EXTRA ?= -s -w # default to building stripped executables
LDFLAGS := ${LDFLAGS_EXTRA} -X github.com/skupperproject/skupper/internal/version.Version=${VERSION}
TESTFLAGS := -v -race -short
GOOS ?= linux
GOARCH ?= amd64

REGISTRY := quay.io/skupper
IMAGE_TAG := v2-dev
ROUTER_IMAGE_TAG := main
PLATFORMS ?= linux/amd64,linux/arm64
CONTAINERFILES := Dockerfile.cli Dockerfile.kube-adaptor Dockerfile.controller Dockerfile.network-observer Dockerfile.system-controller
GO_IMAGE_BASE_TAG := 1.24.7
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

all: skupper controller kube-adaptor network-observer system-controller

basepkg = github.com/skupperproject/skupper
# This lists non-test Go files inside each directory corresponding
# to the first argument and any Go-determined dependencies
godeps = $(filter-out %_test.go,$(wildcard $(patsubst %,%/*.go,$(1) $(shell go list -json $(1) | jq -r '.Deps[] | select(startswith("$(basepkg)/")) | sub("$(basepkg)"; ".")'))))
# This lists embedded files in Go dependencies
embeddeddeps = $(wildcard $(shell grep //go:embed $(call godeps,$(1)) | sed -E 'sX[^/]+.go:.*//go:embed XX'))
# This lists all dependencies from a given package
pkgdeps = $(call godeps,$(1)) $(call embeddeddeps,$(1))

build-cli: skupper
skupper: $(call pkgdeps,./cmd/skupper)
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o $@ ./cmd/skupper

build-controller: controller
controller: $(call pkgdeps,./cmd/controller)
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o $@ ./cmd/controller

build-kube-adaptor: kube-adaptor
kube-adaptor: $(call pkgdeps,./cmd/kube-adaptor)
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o $@ ./cmd/kube-adaptor

build-network-observer: network-observer
network-observer: $(call pkgdeps,./cmd/network-observer)
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o $@ ./cmd/network-observer

build-doc-generator: generate-doc
generate-doc: $(call pkgdeps,./internal/cmd/generate-doc)
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o $@ ./internal/cmd/generate-doc

build-system-controller: system-controller
system-controller: $(call pkgdeps,./cmd/system-controller)
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o $@ ./cmd/system-controller

## native/default container image builds
docker-build: $(patsubst Dockerfile.%,docker-build-%,$(CONTAINERFILES))
docker-build-%: Dockerfile.%
	${DOCKER} build --build-arg GO_IMAGE_BASE_TAG=$(GO_IMAGE_BASE_TAG) $(SHARED_IMAGE_LABELS) -t "${REGISTRY}/$*:${IMAGE_TAG}" -f $< .

podman-build: $(patsubst Dockerfile.%,podman-build-%,$(CONTAINERFILES))
podman-build-%: Dockerfile.%
	${PODMAN} build --build-arg GO_IMAGE_BASE_TAG=$(GO_IMAGE_BASE_TAG) $(SHARED_IMAGE_LABELS) -t "${REGISTRY}/$*:${IMAGE_TAG}" -f $< .

# Push all container images built by podman-build
podman-push: $(patsubst Dockerfile.%,podman-push-%,$(CONTAINERFILES))

# Pattern rule to push individual images
podman-push-%: podman-build-%
	${PODMAN} push "${REGISTRY}/$*:${IMAGE_TAG}"

## multi-platform container images built in docker buildkit builder and
# exported to oci archive format.
multiarch-oci: $(patsubst Dockerfile.%,multiarch-oci-%,$(CONTAINERFILES))
multiarch-oci-%: Dockerfile.% oci-archives
	${DOCKER} buildx build \
		--build-arg GO_IMAGE_BASE_TAG=$(GO_IMAGE_BASE_TAG) \
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

# must-gather base is amd64 only
docker-build-must-gather:
	${DOCKER} build --build-arg GO_IMAGE_BASE_TAG=$(GO_IMAGE_BASE_TAG) $(SHARED_IMAGE_LABELS) -t "${REGISTRY}/skupper-must-gather:${IMAGE_TAG}" -f Dockerfile.must-gather .

docker-push-must-gather:
	${DOCKER} push "${REGISTRY}/skupper-must-gather:${IMAGE_TAG}"

podman-build-must-gather:
	${PODMAN} build --build-arg GO_IMAGE_BASE_TAG=$(GO_IMAGE_BASE_TAG) $(SHARED_IMAGE_LABELS) -t "${REGISTRY}/skupper-must-gather:${IMAGE_TAG}" -f Dockerfile.must-gather .

podman-push-must-gather:
	${PODMAN} push "${REGISTRY}/skupper-must-gather:${IMAGE_TAG}"

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
		-coverpkg=./... \
		-coverprofile cover.out \
		./...

generate-manifest: skupper
	./skupper version -o json > manifest.json

generate-docs: generate-doc
	./generate-doc ./doc/cli

generate-skupper-helm-chart:
	./scripts/skupper-helm-chart-generator.sh ${IMAGE_TAG} ${ROUTER_IMAGE_TAG}

generate-skupper-deployment-cluster-scoped:
	./scripts/skupper-deployment-generator.sh cluster ${IMAGE_TAG} ${ROUTER_IMAGE_TAG} false > skupper-cluster-scope.yaml

generate-skupper-deployment-namespace-scoped:
	./scripts/skupper-deployment-generator.sh namespace ${IMAGE_TAG} ${ROUTER_IMAGE_TAG} false > skupper-namespace-scope.yaml

pack-skupper-helm-chart: generate-skupper-helm-chart
	helm package ./charts/skupper

pack-network-observer-helm-chart:
	helm package ./charts/network-observer

generate-operator-bundle:
	./scripts/skupper-operator-bundle-generator.sh ${IMAGE_TAG} ${ROUTER_IMAGE_TAG}

generate-network-observer-operator-bundle:
	./scripts/skupper-network-observer-operator-generator.sh ${IMAGE_TAG}

generate-network-observer:
	helm template skupper-network-observer ./charts/network-observer/ \
		--set skipManagementLabels=true \
		--set auth.strategy=none \
		> skupper-network-observer.yaml

generate-network-observer-httpbasic:
	helm template skupper-network-observer ./charts/network-observer/ \
		--set skipManagementLabels=true \
		--set auth.strategy=basic \
		> skupper-network-observer-httpbasic.yaml

generate-network-observer-openshift:
	helm template skupper-network-observer ./charts/network-observer/ \
		--set skipManagementLabels=true \
		--set auth.strategy=openshift \
		--set tls.openshiftIssued=true \
		--set tls.skupperIssued=false \
		--set route.enabled=true \
		> skupper-network-observer-openshift.yaml

generate-network-observer-devel:
	helm template skupper-network-observer ./charts/network-observer/ \
		--set auth.strategy=none \
		--set extraArgs={"-cors-allow-all"} \
		--set skipManagementLabels=true > skupper-network-observer-devel.yaml

clean:
	rm -rf skupper controller kube-adaptor \
		network-observer generate-doc \
		cover.out oci-archives bundle bundle.Dockerfile \
		skupper-*.tgz artifacthub-repo.yml \
		network-observer-*.tgz  skupper-*-scope.yaml \
		network-observer-operator \
		must-gather.local.*
