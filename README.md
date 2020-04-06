# [WIP] skupper [WIP]

Note: this repository is a work in progress targeted for the consolidation
and refactoring of the skupper implementation.

Command line tool for setting up and managing skupper installations

## Usage

See `skupper help` or `skupper <command> --help` for details.

## Example

In one kubernetes context do:

```
skupper init
skupper connection-token /path/to/mysecret.yaml
```

In another context, e.g. another kubernetes cluster, do:

```
skupper init
skupper connect --secret /path/to/mysecret.yaml
```

By default skupper will try to set itself up to allow connections from
other skupper sites (using mutual TLS). It uses a LoadBalancer service
or an OpenShift Route for this. If you don't want this, or your
cluster is not set up to support those options, you can use the
`--cluster-local` option to `skupper init` and will then be able only
accept connections from skupper instances in different namespaces on
the same cluster.

Note: if using minikube, you can get the LoadBalancer service to setup
an external ip by running minikube tunnel. (Or else use
--cluster-local as described above).



