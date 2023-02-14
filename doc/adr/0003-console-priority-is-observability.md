# 3. Skupper console provides data first

Date: 2023-02-14

## Status

Accepted

## Context

The previous Skupper console (Gilligan) allowed users to link sites.
With the introduction of the flow collector, there is an opportunity to rework the console to provide a rich environment for network analysis.

## Decision

We will focus on visualizing network traffic in the console. That is, the console becomes a read-only system that typically resides on a single site in the VAN.

## Consequences

Users will no longer be able to link sites using Skupper console.