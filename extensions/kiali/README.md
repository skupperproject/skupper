# skupper-kiali-bridge

An experimental addon for a skupper deployment to expose kiali extension metrics.

## Deploy

This extension needs to be deployed to a kuberentes namespace with an existing
skupper site running. It exposes a `/metrics` http endpoint on port 9000 that
can be scraped by prometheus containing experimental Kiali extension metrics.

Use `deployment.yaml` as a starting point. Applying it to the namespace of a
skupper site in the service mesh cluster should begin to serve the kiali
extension metrics, and should automatically be scraped by the istio prometheus
in the kiali hack/istio/skupper configuration.

```
kubectl apply -f https://raw.githubusercontent.com/c-kruse/skupper/skupper-kiali-bridge/extensions/kiali/deployment.yaml
```

## Metrics

Presently we are only implementing the TCP metrics.

* `kiali_ext_tcp_sent_total`
* `kiali_ext_tcp_received_total`
* `kiali_ext_tcp_connections_opened_total`
* `kiali_ext_tcp_connections_closed_total`

## Labels

A shared set of labels present on each metric.

### Static Labels
* `extension` configurable. defaults to "skupper"
* `reporter` hardcoded to "combined" the flow collector correlates flow events
  from both ends of the connection in order to expose the full context of the
  connection.
* `reporter_id` set to the hostname of this pod

### Not Implemented

* `security`: this is something we _could_ probably find a way to add in
  listener/connector records. So far I don't think it has came up organically.
* `flags`

### Dynamic based on flows
* `source_cluster`, `dest_cluster`: the Skupper Site IDs of the listener (entry
  router) and connector (exit router).
* `source_namespace`, `dest_namespace`: the kubernetes namesapces of the
  listener and connector routers (when applicable, otherise "")
* `source_name`: the hostname of the skupper router pod where client traffic
  was first routed to skupper.
* `dest_name`: the address used to route vanflow traffic.
  This doesn't necessarily correspond to the service name on either end, but it
  is what I had.
