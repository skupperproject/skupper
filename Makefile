VERSION := $(shell git describe --tags --dirty=-modified)
SERVICE_CONTROLLER_IMAGE := quay.io/skupper/service-controller
SITE_CONTROLLER_IMAGE := quay.io/skupper/site-controller
DOCKER := docker

all: build-cmd build-controllers

build-cmd:
	go build -ldflags="-X main.version=${VERSION}"  -o skupper cmd/skupper/skupper.go

build-service-controller:
	go build -ldflags="-X main.version=${VERSION}"  -o service-controller cmd/service-controller/main.go cmd/service-controller/controller.go cmd/service-controller/service_sync.go

build-site-controller:
	go build -ldflags="-X main.version=${VERSION}"  -o site-controller cmd/site-controller/main.go cmd/site-controller/controller.go

build-controllers: build-site-controller build-service-controller

docker-build:
	${DOCKER} build -t ${SERVICE_CONTROLLER_IMAGE} -f Dockerfile.service-controller .
	${DOCKER} build -t ${SITE_CONTROLLER_IMAGE} -f Dockerfile.site-controller .

docker-push:
	${DOCKER} push ${SERVICE_CONTROLLER_IMAGE}
	${DOCKER} push ${SITE_CONTROLLER_IMAGE}

clean:
	rm -rf skupper release

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



