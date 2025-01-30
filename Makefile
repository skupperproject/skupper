VERSION := $(shell git describe --tags --dirty=-modified --always)
REVISION := $(shell git rev-parse HEAD)

LDFLAGS_EXTRA ?= -s -w # default to building stripped executables
LDFLAGS := ${LDFLAGS_EXTRA} -X github.com/skupperproject/skupper/internal/version.Version=${VERSION}
TESTFLAGS := -v -race -short
GOOS ?= linux
GOARCH ?= amd64

REGISTRY := quay.io/skupper
IMAGE_TAG := v2-dev
PLATFORMS ?= linux/amd64,linux/arm64
CONTAINERFILES := Dockerfile.cli Dockerfile.kube-adaptor Dockerfile.controller Dockerfile.network-observer
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

all: build-cli build-kube-adaptor build-controller build-network-observer

build-cli:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o skupper ./cmd/skupper

build-controller:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o controller ./cmd/controller

build-kube-adaptor:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}"  -o kube-adaptor ./cmd/kube-adaptor

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
		-coverpkg=./... \
		-coverprofile cover.out \
		./...

generate-manifest: build-cli
	./skupper version -o json > manifest.json

generate-doc: build-doc-generator
	./generate-doc ./doc/cli

generate-skupper-helm-chart:
	./scripts/skupper-helm-chart-generator.sh ${IMAGE_TAG}

generate-skupper-deployment-cluster-scoped:
	kubectl kustomize ./config/default/cluster > skupper-cluster-scope.yaml

generate-skupper-deployment-namespace-scoped:
	kubectl kustomize ./config/default/namespace> skupper-namespace-scope.yaml

pack-skupper-helm-chart: generate-skupper-helm-chart
	helm package ./charts/skupper

pack-network-observer-helm-chart:
	helm package ./charts/network-observer

generate-bundle:
	./scripts/generate-bundle.sh

generate-network-observer:
	helm template skupper-network-observer ./charts/network-observer/ \
		--set skipManagementLabels=true \
		--set auth.strategy=none \
		> skupper-network-observer.yaml

generate-network-observer-httpbasic:
	helm template skupper-network-observer ./charts/network-observer/ \
		--set skipManagementLabels=true \
		--set auth.strategy=basic \
		--set auth.basic.htpasswd="" \
		> skupper-network-observer-httpbasic.yaml

generate-network-observer-devel:
	helm template skupper-network-observer ./charts/network-observer/ \
		--set auth.strategy=none \
		--set extraArgs={"-cors-allow-all"} \
		--set skipManagementLabels=true > skupper-network-observer-devel.yaml

push-skupper-artifacthub-repo:
	./scripts/push-artifacthub-repo.sh skupper

push-network-observer-artifacthub-repo:
	./scripts/push-artifacthub-repo.sh network-observer

clean:
	rm -rf skupper controller kube-adaptor \
		network-observer generate-doc \
		cover.out oci-archives bundle bundle.Dockerfile \
		charts/skupper skupper-*.tgz artifacthub-repo.yml \
		network-observer-*.tgz  skupper-*-scope.yaml
