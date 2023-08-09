# 4. Move Claims Redemption

Date: 2023-05-18

## Status

Proposal

## Context

Currently claims tokens are converted into certificates (if the claim
is valid) by code embedded in the service-controller. This means that
this functionaly us exposed through a different service and ingress
resource than the certs tokens are used with. This can cause confusion
with configuration where the ingress mode of the console is what is
relevant to the claims.

## Decision

The claims redemption logic will be moved out of service controller
and into the config-sync sidecar in the router deployment. It will
thus be accessed through the skupper-router service and the ingress
mode for actual linking and claims redemption will always be the same.

## Consequences

The skupper-router role needs to be expanded to allow this.

The config-sync sidecar has no expanded beyond the simple syncing of
configuration and should at some point be renamed.
