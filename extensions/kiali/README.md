# skupper-kiali-bridge

An experimental addon for a skupper deployment to expose kiali extension metrics.

## Deploy

This extension needs to be deployed to a kuberentes namespace with an existing
skupper site running. It exposes a `/metrics` http endpoint on port 9000 that
can be scraped by prometheus containing experimental Kiali extension metrics.

Use `deployment.yaml` as a starting point.

## Metrics

Presently we are only implementing the TCP metrics.

* `kiali_ext_tcp_sent_total`
* `kiali_ext_tcp_received_total`
* `kiali_ext_tcp_connections_opened_total`
* `kiali_ext_tcp_connections_closed_total`

## Labels

A shared set of labels present on each metric.

### Static Labels
* `extension` hardcoded to "skupper"
* `reporter` hardcoded to "combined" the flow collector correlates flow events
  from both ends of the connection in order to expose the full context of the
  connection.
* `reporter_id` set to the hostname of this pod

### Not Implemented

* `security`: this is something we _could_ probably find a way to add in
  listener/connector records. So far I don't think it has came up organically.
* `flags`
* `source_version`, `dest_version`

### Dynamic based on flows
* `source_cluster`, `dest_cluster`: the Skupper Site IDs of the listener (entry
  router) and connector (exit router).
* `source_namespace`, `dest_namespace`: the kubernetes namesapces of the
  listener and connector routers (when applicable, otherise "")
* `source_service`, `dest_service`: the address used to route vanflow traffic.
  This doesn't necessarily correspond to the service name on either end, but it
  is what I had. This is probably I think we could use the most thought. It may
  even be worth it to find a way to bring in extra info form the mesh to try
  and match these up with something kiali can better correlate with the istio
  side.

