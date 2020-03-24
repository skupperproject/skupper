FROM golang:1.13

WORKDIR /go/src/app
COPY . .

RUN go mod download

RUN go build -o controller cmd/skupper-controller/main.go cmd/skupper-controller/controller.go cmd/skupper-controller/service_sync.go

CMD ["/go/src/app/controller"]

