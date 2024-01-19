# 7. Deprecate ServiceSync

Date: 2024-01-16

## Status

Proposed

Enabled By [6. Explicit resources for declaring service bindings](0006-explicit-resources-for-declaring-service-bindings.md)

## Context

ServiceSync is a mechanism whereby service ingress bindings are
automatically created at each site in the network for any service with
egress bindings.

Though this feature seemed nice in early demos, in practice it is
generally unnecessary. In some cases it is explicitly not desirable as
the ingress bindings should only be created in particular sites.

Though the feature can at present be turned off, the option to do so
is not obvious. Further it is not well tested.

If the decision to have explicit resources for ingress bindings is
adopted, it becomes even simpler to explicitly add the required
ingresses to any site that requires them. Indeed as that proposal
allows more flexibility with regard to the ingress bindings
ServiceSync would not be adequate as it is.

## Decision

ServiceSync will not be supported for automatic propoagation of
ingress bindings between sites that have migrated to use explicit
resources for each ingress binding.

## Consequences

This will require extra declarations to be made by user at the sites
at which they want to consume services. However this also gives much
more flexibility not only in the location of the ingress bindings, but
also in their configuration.

Upgrading a network that currently relies on ServiceSync may be more
awkward if this decision is accepted. It would be possible to
alleviate some of that by for example providing a legacy support
process that would turn the new model into the expected sequences for
the old ServiceSync protocol. What level of effort is appropriate for
easing upgrade will be decided through separate ADRs.

This change would also impact existing examples, both under the
skupperproject organisation and elsewhere.
