VERSION := $(shell git describe --tags --dirty=-modified)
IMAGE := quay.io/ajssmith/skupper-proxy-controller

all: build-cmd build-controller

build-cmd:
	go build -ldflags="-X main.version=${VERSION}"  -o skupper cmd/skupper/skupper.go

build-controller:
	go build -ldflags="-X main.version=${VERSION}"  -o controller cmd/skupper-controller/main.go cmd/skupper-controller/controller.go cmd/skupper-controller/service_sync.go

docker-build:
	docker build -t ${IMAGE} .

docker-push:
	docker push ${IMAGE}

clean:
	rm -rf skupper release

deps:
	dep ensure

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



