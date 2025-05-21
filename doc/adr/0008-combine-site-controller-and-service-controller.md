# 8. Combine site-controller and service-controller

Date: 2024-01-16

## Status

Proposed

## Context

The site-controller was introduced as a way of allowing declarative
configuration of skupper for a namespace in Kubernetes. It watched for
the skupper-site ConfigMap and then used essentially the same code as
was used by the CLI to initialise the namespace. A new separate
process was the least disruptive way to support declarative use at
that time.

There have been two changes that make it easier to revisit this
configuration.

The first is the removal of the original console which was tied to the
service-controller. The flow-collector and the current console can be
deployed quite independent of the service-controller now.

The second is the introduction of the config-sync container and the
implementation of a leader election among instances if there are more
than one. This can be used to relocate any code that requires direct
connection to the router.

## Decision

A new controller will be created that will combine the functions of
the current site- and service- controllers. This new controller will
be able to run either watching all namespaces or watching the current
namespace, similar to the current site-controller.

The controller will also support updating the site in line with any
configuration changes.

Features that require connection to the router will be moved into the
config-sync container.

## Consequences

Removing an unnecessary component means simpler configuration and
simpler code for automating that configuration. With this change the
controller is concerned only with the configuration of the data
plane. The configuration of the controller itself can be achieved by
customing the yaml (as is currently the case for the site-controller).

Skupper will also use one less pod per site in many cases.

The get command and associated web api will no longer be available in
its current form. Neither will the already deprecated web api for
managing links and tokens. The preferred approach for making
information available will be through kubernetes resources. An
alternative to `skupper debug events` will be needed. This might be
Kubernetes Event resources or some other resource into which
information can be written.

Upgrade from the current configuration to this new one will need some
consideration.
