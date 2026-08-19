# 11. Compressed router configuration for Kubernetes sites

Date: 2026-08-15

## Status

Proposed

Supersedes the chunked router configuration proposal
([skupperproject/skupper#2557](https://github.com/skupperproject/skupper/pull/2557)).

## Context

On Kubernetes, the controller serializes the complete desired router
configuration into a single ConfigMap per router group (`skupper-router`,
`skupper-router-2`). The kube-adaptor and the config-init container consume it:
config-init renders it as `skrouterd.json` so routers cold-start with full
configuration, and the kube-adaptor applies changes to running routers over
AMQP management.

A ConfigMap holds at most 1MiB. Sites in the field already approach this
ceiling. There is no bound on the number of Listener-like, Connector-like,
Link, or RouterAccess resources a Site can be configured with, and the
environmental limits (available file descriptors, ports, Services) sit far
beyond what 1MiB of plaintext configuration can represent. The transport is
therefore the first limit users hit.

Router configuration JSON is highly repetitive and compresses well in practice.
Depending on the workload, this can be a 5x to 20x reduction in total bytes.

## Decision

On Kubernetes, continue storing the router configuration plain below a
configurable threshold, and store it gzip-compressed in the same ConfigMap
when its serialized size reaches or exceeds that threshold. The default
threshold is 896KiB; a value of zero or less disables compression.

**Below the threshold, the transport is unchanged.** The configuration is
stored plain under the existing `skrouterd.json` data key in the same format
consumers read today. This remains the representation for the majority of
sites, keeping typical configurations directly inspectable and maximizing
compatibility.

**At or above the threshold, the configuration is stored compressed** in
`binaryData` under the key `skrouterd.json.gz`, encoded with gzip at maximum
compression. Exactly one representation is present at a time.

**The threshold is configurable and defaults high.** The controller flag
`-router-config-compression-threshold` (environment variable
`ROUTER_CONFIG_COMPRESSION_THRESHOLD`) sets the size in bytes at which the
configuration is written compressed. Compression requires compression-aware
readers. The 896K default leaves 128KiB below the ConfigMap limit while
keeping all but the largest Sites readable by pre-change versions after a
downgrade; only Sites likely to encounter the limit trade compatibility for
capacity.

**All readers understand both representations.** The controller, config-init,
and the kube-adaptor read the compressed key when present and fall back to the
plain key otherwise. The router itself is unaffected: it consumes the plain
`skrouterd.json` file that config-init renders.

**Debug tooling stays useful.** `skupper debug dump` archives a decompressed
copy of `skrouterd.json` alongside the ConfigMap when the configuration is
stored compressed. Outside the dump, the gzip format keeps configurations
inspectable with universal tooling:

```
kubectl get cm skupper-router \
    -o jsonpath='{.binaryData.skrouterd\.json\.gz}' | base64 -d | gunzip
```

Non-Kubernetes platforms are unaffected: they keep a complete plain
configuration file, and compression remains Kubernetes transport packaging
only.

## Consequences

* Compression raises but does not remove the ConfigMap limit. If the compressed
  configuration still exceeds 1MiB — or the plain configuration exceeds it
  while compression is disabled — the API server rejects the ConfigMap write;
  there is no spill or chunking fallback. The practical ceiling is
  workload-dependent: on the order of 15,000 selector targeted pods, 10,000
  Listener services per site.
* Pre-change consumers remain compatible only while the ConfigMap uses the
  plain representation. Normal upgrade rollout ordering minimizes the
  version-skew window by updating router deployments before configuration
  updates; the high default additionally preserves downgrade compatibility for
  most Sites. Manual steps to disable compression may be required before
  downgrading to a version without compression support.
* For compressed sites, ConfigMap storage and watch payloads shrink 5-10x,
  although every configuration change still rewrites the full object.
* With the high default, medium-sized configurations remain directly
  inspectable but forgo compression's API server and etcd savings; the
  default deliberately favors compatibility and inspectability. Once all
  supported versions read both representations, the compatibility rationale
  expires and a later release may lower the default substantially or fix it
  at a static low value.
* Compressed configurations are no longer directly human-readable in the
  ConfigMap; debug tooling compensates as described above.

## Alternatives

**Chunked configuration** (the superseded proposal) splits the configuration
across a head document and immutable, content-addressed chunk ConfigMaps,
removing the transport bound entirely. It remains the more complete answer if
the residual ceiling is ever reached in practice, and compression does not
prevent adding it later.

**Limiting Site configuration** would preserve the current transport by
imposing a fixed router-configuration budget. Enforcing and reporting that
budget is complicated because selector Connectors and exposePodsByName
Listeners have dynamic footprints that change as workloads change. A status
warning as the compressed size approaches the limit remains worthwhile
independent of this decision.

**Storing configuration in resource annotations or status** would avoid the
single ConfigMap limit. Using annotations is risky as it inverts ownership,
may conflict with GitOps systems, and competes for a relatively small
annotation size limit. Adding the configuration serialization to the resource
status clouds the user API with internal implementation details.

**An xDS-style controller API** would allow any amount of configuration to be
served directly from the controller to the router deployment, but introduces
new uptime requirements for the controller. Having the full configuration
available in-cluster for cold-starting router Pods is ideal.

**Moving configuration into an optional persistent database backend** would
remove the ConfigMap transport limit, but requires users to manage a
persistent workload on Kubernetes (or bring their own).
