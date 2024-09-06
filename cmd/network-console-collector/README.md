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

The network console collector exposes a set of prometheus metrics alongside the
API at `/metrics`.

### Application Network Traffic

This set of metrics exposes details about service traffic though the skupper
network. These metrics have a shared set of labels that allow us to expose
traffic patterns on a per client/service basis.


Signals:

All metric names are prefixed with the `skupper` namespace.

| metric name | description |
| ------------------------ | ------------------------  |
| connections_opened_total | Number of connections opened through the application network |
| connections_closed_total | Number of connections opened through the application network that have been closed |
| sent_bytes_total         | Bytes sent through the application network from client to service |
| received_bytes_total     | Bytes sent through the application network back from service to client |

Dimensions:

| label name | description |
| -------------- | ------------------------  |
| source_site_id | ID of the site where the connection was established |
| source_site_name | Name of the source site |
| dest_site_id | ID of the site where the connection exited the skupper network through a connector |
| dest_site_name | Name of the distination site |
| routing_key | The routing key of the service |
| protocol | The protocol used in the exchange (TCP) |
| source_process | The name of the client process as reflected in the collector API |
| dest_process | The name of the server process as reflected in the collector API |

### Application Network Request Traffic (Application Layer)

A proposed (unimplemented) set of metrics that expose details about HTTP and
HTTP/2 exchanges when enabled in the router.

Signals:

Like application traffic metrics, these are prefixed with `skupper`.

| metric name | description |
| ------------------------ | ------------------------  |
| requests_total | Counter incremented for each request handled through the skupper network |

Dimensions:

Request metrics share the same set of dimensions as the network traffic metrics
above, but includes several additional labels.

| label name | description |
| -------------- | ------------------------  |
| method | HTTP request method |
| code | HTTP response code class (for example, a response code 201 would be counted towards code='2xx') |

### Internal Metrics

We expose a set of metrics prefixed `skupper_internal` to help us observe the
collector itself. Moving forward we may end up recommending anyone scraping
skupper metrics from the collector exclude these when storage space is a
concern.


Related to Network Traffic metrics (and sharing the same label set.)

| metric name | description |
| ------------------------ | ------------------------  |
| latency_seconds | Histogram of connection latency observations - the difference between TTFB on the listener and connector sides. TCP only. |

Primarily for backward compatibility with the skupper v1 network console. New
users should prefer the `latency_seconds` signal. The
`legacy_flow_latency_microseconds` metric exposes the Time to First Byte (TTFB)
as observed by skupper from the client (listener) and server (connector) sides
on TCP connections. This metric shares the same standard set of labels
described in the Application Network Traffic section, but adds a `direction`
label to differentiate the listener and connector side TTFB.

| metric name | description |
| ------------------------ | ------------------------  |
| legacy_flow_latency_microseconds | Histogram of connection time to first byte observations in microseconds. TCP only. |
