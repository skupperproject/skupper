FROM golang:1.13

WORKDIR /go/src/app
COPY . .

RUN go mod download

RUN go build -o controller cmd/service-controller/main.go cmd/service-controller/controller.go cmd/service-controller/service_sync.go

CMD ["/go/src/app/controller"]

