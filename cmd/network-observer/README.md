# Network Observer

The network observer is an application that attaches to the skupper router
network in order to expose skupper network telemetry. When installed alongside
a skupper site it will collect operational data from ALL sites in the network
and expose them via the API and metrics that back the [Skupper
Console](https://github.com/skupperproject/skupper-console) web application.

## Deployment

The Network Observer can be deployed with the
[network-observer](../../charts/network-observer/README.md) Helm Chart.

## API

The Skupper Network Console HTTP API is described in an openapi 3.0
specification file inside the `spec` directory. To view the API, either import
this document into an online swagger editor, or start the collector locally and
access the Swagger UI at `http://localhost:8080/swagger/`.

To edit the specification using a hosted (swagger
editor)[https://editor-next.swagger.io/], open the editor in your browser, and
import the spec by URL (File -> Import URL) from
`https://raw.githubusercontent.com/skupperproject/skupper/v2/cmd/network-observer/spec/openapi.yaml`.

## Metrics

The network console collector exposes a set of Prometheus metrics alongside the
API at `/metrics`.

### Operational Site Metrics

This set of metrics exposes coarse site-level details pertaining to the
operation and topology of the skupper network.

* `skupper_site_info`: Metadata about the active sites in the network. Labels are `site_id`, `name` and `version`.
* `skupper_routers_total`: Number of active routers in each site. Labels are `site_id` and `mode`.
* `skupper_site_links_total`: Number of links from each site. Counts all router
  links so may not map directly to the number of links.skupper.io resources in
  the case of HA deployments. Labels are `site_id`, `status` and `role`.
* `skupper_site_link_errors_total`: Count of link connection errors from
  routers in a site. Labels are `site_id` and `role`.
* `skupper_site_listeners_total`: Number of listeners on each site. Counts all listeners
  so may not map directly to the number of listeners.skupper.io resources in
  the case of HA deployments. Label is `site_id`.
* `skupper_site_connectors_total`: Number of connectors on each site. Counts
  all connectors so may not map directly to the number of connectors.skupper.io
  resources in the case of HA deployments or connectors with a selector
  matching any number of pods. Label is `site_id`.

### Application Network Traffic Metrics

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
| dest_site_name | Name of the destination site |
| source_component_id | ID of the process group for the client process establishing the connection |
| source_component_name | Name of the source process group |
| dest_component_id | ID of the process group for the server process |
| dest_component_name | Name of the destination process group |
| routing_key | The routing key of the service |
| protocol | The protocol used in the exchange (TCP) |
| source_process_name | The name of the client process as reflected in the collector API |
| dest_process_name | The name of the server process as reflected in the collector API |

### Application Network Request Traffic Metrics (Application Layer)

A proposed set of metrics that expose details about HTTP and HTTP/2 exchanges
when enabled in the router.

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
concern. All internal metrics should be considered unstable.


**notable internal metrics to the user**

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
