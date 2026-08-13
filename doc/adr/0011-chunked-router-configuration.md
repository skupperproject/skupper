# 11. Chunked router configuration for Kubernetes sites

Date: 2026-08-12

## Status

Proposed

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
environmental limits (available file descriptors, ports) sit far beyond what
1MiB of configuration can represent. The transport is therefore the first
limit users hit.

Additionally, every configuration change rewrites the full object: a site
with 500KiB of configuration pays a 500KiB read, write, and watch event for a
one-line change, resulting in a meaningful API server and etcd load at scale.

## Decision

Extend the router configuration scheme with a spill-over transport:

**Below a size threshold, the transport is unchanged.** The configuration is
stored under the existing `skrouterd.json` data key in the same format
consumers read today; no consumer changes are needed to handle it. This remains
the representation for most sites with small configurations.

**Above the threshold, the configuration spills into chunks.** The
`skupper-router` ConfigMap instead carries a small *head* document that names
the complete configuration by referencing set of *chunk* ConfigMaps:

* Configuration is decomposed into keyed records (one per router entity).
* Records are partitioned deterministically into chunks by hashing their keys;
  chunk count is derived from total size.
* Chunks are immutable (`ConfigMap.immutable: true`) and content-addressed: named and
  referenced by the digest of their content.
* The head lists the digests of the complete chunk set.

**Publishing is ordered for atomicity.** The controller creates any missing
chunks first, then updates the head in a single object write. The head update
is the atomic boundary: a reader that follows head → chunks always assembles
one consistent generation. Readers treat a missing chunk (deleted under them)
as a signal to re-read the head and retry.

**Unreferenced chunks are garbage-collected** after a grace period.

**Serialization becomes deterministic.** One logical configuration has exactly
one byte representation: Records are serialized as compact JSON in a fixed
order by key. Readers merge records from all chunks and apply this ordering, so
the inline and chunked representations produce identical skrouterd.json bytes.
Deterministic chunk serialization ensures logically identical chunk content has
the same digest.

Config-init and the kube-adaptor understand both representations; their
downstream behavior (complete config file, ordered AMQP reconciliation) is
unchanged. Non-Kubernetes platforms are unaffected: they keep a complete
configuration file, and chunking remains Kubernetes transport packaging only.

## Consequences

* Configuration size is no longer bounded by the transport. The effective
  limits return to the router and the environment.
* Because the inline representation keeps the current data key and format,
  sites below the threshold need no consumer changes, and version skew during
  upgrade is only a concern for spilled configurations. A configuration that
  has spilled cannot be downgraded to a release without chunk support.
* The spill threshold starts high (spill as last resort, maximizing backwards
  compatibility) and may be lowered in a later release as a performance
  measure.
* Spilled configurations are no longer directly human-readable in the
  ConfigMap. Debug tooling must assemble head + chunks and print the
  equivalent `skrouterd.json`.

## Alternatives

**Limiting Site configuration** would preserve the current transport by imposing
a fixed router-configuration budget. Enforcing and reporting that budget is
complicated because selector Connectors and exposePodsByName Listeners have
dynamic footprints that change as workloads change. Static resource-count
limits are also impractical: only 13 MultiKeyListeners, each using the maximum
256 routing keys of 64 bytes, exceed the 1 MiB limit.

**Compression** stretches the limit but does not fix the underlying issue.
With compression there is still a need to **Limit Site configuration.**

**Storing configuration in resource annotations or status** would avoid the
single ConfigMap limit. Using annotations is risky as it inverts ownership, may
conflict with GitOps systems, and competes for a relatively small annotation
size limit of 256 KiB. Adding the configuration serialization to the resource
status fixes the ownership issue, but clouds the user API with internal
implementation details.

**An xDS-style controller API** would allow any amount of configuration to be
served directly from the controller to the router deployment. This would
introduce new uptime requirements for the controller. Having the full
configuration available in-cluster for cold-starting router Pods is ideal.

**Moving Configuration into an optional persistent database backend** would
solve a lot of problems, but requires users to manage a persistent workload on
Kubernetes (or bring their own.)
