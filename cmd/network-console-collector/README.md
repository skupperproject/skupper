# Network Console Collector

The network console collector is the main component of the network console
application, serving as a bridge between skupper network telemetry and the
user. When installed alongside a skupper site it will collect operational data
from ALL sites in the network and expose them via the API and metrics that back
the [Skupper Console](https://github.com/skupperproject/skupper-console) web
application.

## Deployment

Deployment examples can be found in the [./resources](./resources/README.md)
directory. These is still under development and should not be considered
stable.

## API

Status: dev/doc only (server not in full compliance)

The Skupper Network Console HTTP API is described in an openapi 3.0
specification file inside the `spec` directory. To view the API, either import
this document into an online swagger editor, or start the collector locally and
access the Swagger UI at `http://localhost:8080/swagger/`.

To edit the specification using a hosted (swagger
editor)[https://editor-next.swagger.io/], open the editor in your browser, and
import the spec by URL (File -> Import URL) from
`https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/network-console-collector/spec/openapi.yaml`.

### Code generation

The code generated based off of the openapi specification document provides an
API Client (presently used for unit testing the service) as well as a Server
Interface for us to implement. To update the generated code after updates to
the openapi specification run `go generate` targeting this package. This should
update the types_gen.go and extras_gen.go files in the internal/api package.

## Metrics

TBD
