# 3. Show incoming links as optional

Date: 2023-05-10

## Status

Accepted

## Context

The flow-collector will address situations where requesting remote information, such as network or link status, results in a timeout.

## Decision

To minimize timeouts, remote link information will be optional. 
The default timeout will be reduced from 2 minutes to 5 seconds.

## Consequences

A new flag called `show-remote` will display remote link information in the `link status` command.
