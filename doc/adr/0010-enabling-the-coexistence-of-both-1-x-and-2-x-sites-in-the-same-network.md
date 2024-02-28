# 10. Enabling the coexistence of both 1.X and 2.X sites in the same network

Date: 2024-02-28

## Status

Proposed

## Context

Some users would prefer to transition to Skupper v2.X one site at a time, instead of
deleting their current skupper network and starting a new one.

## Decision

Skupper 1.X sites should be compatible with Skupper 2.X sites within the same network.

## Consequences

It would be necessary to find a strategy for all the actions that rely on the former service-controller that are not 
going to be present in the v2.x version (i.e., service-sync). 

Additionally, it will be necessary to address any potential incompatibility between the former and the new tcpAdaptor 
operating on different routers within the network.