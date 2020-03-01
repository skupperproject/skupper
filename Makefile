VERSION := $(shell git describe --tags --dirty=-modified)

all: build

build:
	go build -ldflags="-X main.version=${VERSION}"  -o skupper cmd/skupper/skupper.go

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



