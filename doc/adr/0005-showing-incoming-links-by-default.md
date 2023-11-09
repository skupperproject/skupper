# 5. showing incoming links by default

Date: 2023-09-13

## Status

Proposal

Supersedes [3. Show incoming links as optional](0003-show-incoming-links-as-optional.md)

## Context

The collector-lite will be available to provide information about the network sites, by writing it in a local configmap.

## Decision

The incoming links will be shown by default again in the link status command.

## Consequences

Link status command must query the configmap instead of the service controller for that.
