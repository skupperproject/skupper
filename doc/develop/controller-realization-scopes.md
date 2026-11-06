# Controller realization scopes

A realization scope is a grouping of output resources and fields owned by the
skupper controller and the input resources that must be considered together in
order to produce those outputs. The principal scope, the Site scope, encloses
everything pertaining to the operation of an individual Site namespace: the
router deployment and configuration, endpoint types, listener services, and all
inputs required to produce it.

**Status: In Progress.** This document describes realization scopes as intended. An incremental
migration is tracked in #2616.

## The Skupper Controller High Level Description

The skupper controller uses familiar kubernetes controller types and patterns
in order to do work. It DOES NOT use the popular controller runtime packages,
but instead uses the primitives in client-go.

When the controller starts, it registers a collection of Informers that
register watchers with the kubernetes API for collections of resources,
synchronizes a local cache of those resources, keeps them in sync and, fires
callbacks for change events. When the controller gets a callback for one of
these resource events it enqueues a work item (key, handler) in a deduplicating
work queue. A single worker runs on a loop pulling and handling work items
sequentially.

### Resource Scope Routing

Resource event handlers perform minimal work. They evaluate the resource that
changed to determine which scope(s) it may effect. For example, a Link in
namespace "east" concerns the Site scope for namespace "east". Relevant scopes
are then queued with their handler, and the resource handler returns.

### Scope Handlers

Scope handlers follow a structure meant to reduce potential for side effects,
aide in testability, and minimize API resource utilization.

* **Snapshot** the synchronized informer caches and copy all observed inputs
  and outputs.
* **Derive** the desired state of output resources and fields from the frozen
  snapshot.
* **Plan** a minimal set of mutations to realize the desired state.
* **Execute** performs API operations, credential generation, runtime
  configuration changes, etc.
* **Log** actions attempted and their outcomes.

## Site Scope

**Key:** namespace. Only one active Site per namespace.

**Inputs:** All Site resources in the namespace, Listener, MultiKeyListener,
Connector, Link, RouterAccess, AttachedConnectorBindings and eligible
AttachedConnectors, Connector targeted selector Pods, router Pods, the
skupper-network-status ConfigMap, TLS and proxy Secrets, router configuration
ConfigMaps (for port allocation), router Deployments, Services and RBAC, Certificate
and SecuredAccess, sizing, metadata, AccessTokens.

**Outputs:** router Deployments with Pod template, router configuration
ConfigMaps, generated ServiceAccount, Role and RoleBindings when Site does not
specify a ServiceAccount. Local router and listener Services. Site
Certificates. RouterAccess when Site is configured with linkAccess.Site,
SecuredAccess, endpoint/Link/RouterAccess status. Annotations of certain
tlsCredentials Secrets.

**Preserves** existing active Site, service port mappings, ssl profile ordinals
and oldest-valid ordinal.

## Certificate Scope

**Key:** Certificate namespace + name. Certificates are evaluated on their own.

**Inputs:** Certificate spec, owner references and annotations. The same-name
Secret resource. CA Secret. The current time.

**Outputs:** The same-name Secret data and metadata. Certificate status.

**Preserves** Certificate key data when still valid.

## SecuredAccess Scope

**Key:** SecuredAccess namespace + name. SecuredAccesses are evaluated on their own.

**Inputs:** SecuredAccess spec, owner references. SecuredAccesses with matching
namespace and spec.certificate. Same-name Service and provider resources
(Route, Ingress, HTTPProxy, TLSRoute.) Certificates. Controller access type
configurations, Gateway.

**Outputs:** Exposure Service and provider resources, Certificate.

## AccessGrant

**Key:** AccessGrant namespace + name.

**Inputs:** AccessGrant, Site, Controller grant-server URL/CA configurations,
time for initialization/expiry.

**Outputs:** AccessGrant status

## Common Inputs: Controller Context

Some controller scoped inputs are common to all scopes: namespace control,
labels and annotation policy, grants server configurations. These are packaged
as controller context and are snapshotted along with scope input. Changes to
controller context may trigger dependent scopes.
