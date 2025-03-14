# 10. Enabling the coexistence of both 1.X and 2.X sites in the same network

Date: 2024-02-28

## Status

Proposed

## Context

The 2.X version of Skupper will incorporate new TCP connectors/listeners from an updated skupper-router version, which are not compatible with the TCP connectors/listeners from the skupper-router version included in Skupper 1.X.

## Decision

Skupper 1.X sites will not be compatible with Skupper 2.X sites within the same network.

## Consequences

If a user wishes to upgrade their Skupper network from version 1.X to 2.X, they should upgrade all sites individually. 
