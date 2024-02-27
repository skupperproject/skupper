# 9. Use CRDs in declarative interface

Date: 2024-02-26

## Status

Proposed

## Context

The motivation for this decision is a desire to provide a better, more
intuitive interface between the controller and the user. Custom
Resource Definitions are now the standard way of providing extended
interfaces in Kubernetes. They are as good as any other format for
podman based sites.

In addition to allowing the user to configure skupper through more
clearly defined types, CRDs also give us a familiar way to get
information back from the controller to the user through the status
sections.

Only the controller image should be tied to using the CRDs
directly. The config-sync image should not need to interact with
them. That ensures that should other aprpoaches be necessary we
minimise the amount of work required.

## Decision

Instead of creating ConfigMaps and Secrets with annotations users will
create custom resources to configure Skupper.

## Consequences

Standard RBAC can be used to limit, at a coarse granularity, what
aspects of Skupper users of a cluster are free to configure.

For example, if a user is denied permission to create Site resources,
then they will be unable to initialise skupper for a particular
namespace. If they are denied the permission to create Link resources,
they will not be able to link a Site to any other. If they are denied
the permission to create Connector resources, they will not be able to
expose workloads from that namespace. If they are denied permission to
create Listener resources they will not be able to offer
Skupper-backed services to that namespace.

The use of CRDs also allows basic rules about what is allowed to be
enforced in a way that is not easy with annotations and labels. As an
example we can define the connector resource to require values for
port and routingKey and to only accept one of host or selector, not
both. This makes it possible to avoid certain misconfigurations.

The controller can also use the status section of defined CRDs to
provide information back to the user, e.g. regarding errors or
indicating that configuration has been applied. This allows a better
user experience using just the standard kubectl tool.

For example, invoking `kubectl get sites -A` on a cluster will show
all the namespaces in which skupper has been enabled. We can also make
use of the 'additional printer columns' feature to show high level
information about each site. Likewise the list of outgoing links
configured, the connectors that expose workloads or the listeners that
define skupper backed services offered can all be listed, either for a
specific namespace or for the cluster as a whole.

It will not be possible to use Skupper without first installing the
CRDs, which requires elevated permissions. This may be mitigated on
OpenShift when using the OLM based operator which will install the
CRDs when the operator is enabled. For some users this will in fact be
a feature, meaning that Skupper cannot be used without explicitly
enabling it for the cluster. (This of course does not prevent
configuring the router directly).

We will also need to provide a client library for these types as
anyone who is programmatically creating them will expect it.
