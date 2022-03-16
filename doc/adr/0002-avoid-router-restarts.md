# 2. Avoid Router Restarts

Date: 2022-03-16

## Status

Accepted

## Context

The creation of links requires to restart the router to append a volume with the associated secret.
Creating or exposing services with the TLS flag enable also requires to restart the router for appending new volumes.
The possibility of frequent router reboots can lead to several interruptions in Skupper operation.

## Decision

We will avoid restarts when creating links or when enabling TLS in Skupper services.

## Consequences

The config-sync sidecar will be leveraged to make the certification files available to 
the router using a shared volume with the latter.

The config-sync sidecar will update not only the bridge configuration, but also the connectors needed for new
links.
