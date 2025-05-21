# 6. Explicit resources for declaring service bindings

Date: 2024-01-16

## Status

Proposed

Enables [7. Deprecate ServiceSync](0007-deprecate-servicesync.md)

## Context

Service bindings are the configurations that determine how a Skupper
service is invoked (ingress binding) and what processes that
invocation may be routed to (egress binding).

The Deployments or StatefulSets that are to be annotated may
themselves be the generated output of other controllers. In this case
it is not always possible to ensure that they are annotated correctly
for Skupper.

The way annotations work when applied to Service resources is
confusing and unintuitive.

If the skupper.io/proxy annotation is found on a Service, the egress
binding is configured based on the original selector in the Service
and this selector is then changed to point the service at the router
pods. This changing of the original resource can cause problems when a
GitOps approach is used for deployment.

However if the skupper.io/target annotation is also applied to the
Service, the behaviour is different. Here the annotated service is
updated to point at the skupper router pods, and the value of target
is used to configure the egress binding.

If instead the skupper.io/ingress-only annotation is also present on
the Service, no target is inferred. The service is configured to point
at the router pods on a port that will route to the address specified
by the skupper.io/address annotation.

The code accumulated to handle all these configurations as well as
the CLI is not the easiest to maintain.

## Decision

Explicit resources will be introduced that describe the desired
configuration for both ingress- and egress- bindings.

## Consequences

Support for the existing annotations, such as skupper.io/proxy, will
be removed. It may be separately decided to provide legacy support in
some fashion that would derive the new explicit bindings from
annotated resources.

It is expected that a model made explicit in the resources used will
be easier for users to understand and internalise, making it more
likely that they are successful in their use of skupper and reducing
their need to seek help with simple configuration.

Explicit bindings resources will also give more flexibility, for
example allowing more than one ingress binding for the same service if
needed, or allowing the hostname used for the ingress to vary by site.

However a change to the model for service bindings will make upgrade
and backward compatibility more of a challenge.

The change in model will need to be clearly documented and explained
to users, along with the recommended path for migration to this new
model.

The change would also impact existing examples, both under the
skupperproject organisation and elsewhere.
