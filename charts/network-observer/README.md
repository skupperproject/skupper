# Skupper Network Observer

The skupper network observer is an application that attaches to the skupper
network in order to expose skupper network telemetry. When installed alongside
a skupper site it will collect operational data from ALL sites in the network
and expose them via the API and metrics that back the [Skupper
Console](https://github.com/skupperproject/skupper-console) web application.

This chart bootstraps a skupper network observer deployment on a
[Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh)
package manager.

## Prerequisites

- A kubernetes cluster or namespace with the skupper controller installed
- A skupper Site

