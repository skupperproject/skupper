# Bootstrapping non-kubernetes sites

In this current phase of the Skupper V2 implementation, non-kubernetes sites
can be bootstrapped using a locally built binary, that can be produced by running
`make build-bootstrap`, or using the (eventually outdated) quay.io/skupper/bootstrap
container image by calling `./cmd/bootstrap/bootstrap.sh`.

Non-kubernetes sites can be created using the standard V2 site declaration
approach, which is based on the new set of Custom Resource Definitions (CRDs).

The same CRDs used to create a V2 Skupper site on a Kubernetes cluster can be
used to create non-kubernetes sites. Some particular options and fields are
valid or required only on specific platforms, but overall the structure is the
same.

Differently than the current Kubernetes implementation, non-kubernetes sites
are static. Meaning that you must have all your Custom Resources (CRs) ready
at the time your site is bootstrapped and if something needs to change, you
will need to remove and bootstrap your site again.

## Supported platforms

You can specify the target platform to be bootstrapped by exporting
the `SKUPPER_PLATFORM` environment variable, using one of the following
allowed values:

* podman
* docker
* systemd
* bundle
* tarball

### Container engine platforms

When `podman` or `docker` platform is used, then the bootstrap procedure will
require that the respective container engine endpoint is available. The default
unix socket will be used based on the current user and platform selected.

### Systemd

The `systemd` platform actually requires that you have a local installation of
the `skupper-router` (`skrouterd` binary must be available in your PATH).

### Site bundle strategies

If you do not want to run an actual site from your CRs, but instead, you just
need to produce a bundle that can be installed somewhere else, then you can
use `bundle` or `tarball` as the platform.

The `bundle` platform will produce a self-extracting shell archive that can
be executed to install your local non-kubernetes site.

If you are not comfortable executing the produced script, you can also choose
`tarball` as the platform. It produces a tar ball that contains a `install.sh`
script, that basically performs the same procedure of the `bundle`.

Both scripts accept flags:

```
-h               help
-p <platform>    podman, docker, systemd
-x               remove
-d <directory>   dump static tokens
```

## Usage

### Bootstrapping and removing non-kubernetes sites

Place all your Custom Resources (CRs) on a local directory.
You must always provide a `Site` (CR), plus some other resource
that makes your running site meaningful, like Listeners and/or Connectors.

In case you want your local site to listen for incoming links from other sites,
at present, you have to explicitly provide a `RouterAccess` (CR).

If you want your non-kubernetes site to establish links to other sites, make
sure you also provide `AccessToken` or `Link` (CRs).

***NOTE:** A V2 representation of the **"Hello world example"** is available below
to provide initial guidance.*

Now that you have all your CRs placed on a local directory, just run:

#### Bootstrapping

```shell
# To bootstrap your site using the binary
export SKUPPER_PLATFORM=podman
./bootstrap ./
```

Alternatively you can also run the `bootstrap.sh` script:

```shell
# To bootstrap your site using the shell script
export SKUPPER_PLATFORM=podman
./cmd/bootstrap/bootstrap.sh ./
```

Remember that you can set the `SKUPPER_PLATFORM` environment variable to
any of the platforms mentioned earlier.

After the bootstrap procedure is completed, it will provide you some relevant
information like:

* Location where static tokens have been defined (when a `RouterAccess` is defined)
* Location where the site bundle has been saved (when using `bundle` or `tarball`)

#### Site bundles

When a site bundle is produced, the bootstrap procedure will point you
to the location where it has been saved.

#### Removing

To remove your site, you can run a local script, that takes the site name
as an argument. The platform is identified by the script so you don't need
to export it.

```shell
./cmd/bootstrap/remove.sh <site-name>
```

## Example

Here is a very basic demonstration on how you can run the `Hello World` example
locally using just non-kubernetes sites.

The example is assuming you are running both `west` and `east` sites on your
machine. So if you want to run it on different machines, make sure to adjust
IP addresses properly.

### Hello world

This example basically runs a **frontend** application on the `west` site,
which depends on a **backend** that is meant to run on the `east` site.

To simulate it, the frontend application will be executed using podman,
it will be exporting port 7070 in the loopback interface of the host machine,
and we will tell it to expect that the backend service will be available
at `http://host.containers.internal:8080`, which from inside the container,
means the host machine at port 8080.

The backend will also run on the host machine with podman, and it will be
exporting port 9090 to the loopback interface of the host machine. So after
both `frontend` and `backend` containers are running, they won't be able
to communicate.

We will use two Skupper sites to resolve that. On the `west` site, Skupper
will expose a `Listener` (CR), bound to port `8080` of the host machine.

On the `east` site, we will have a `Connector` (CR) that targets localhost
at port 9090, in the host machine.

The ideal scenario would be to run each component on different machines,
where Skupper could add real value, but just for the purpose of making it
simple to run, the example is defined to work at a single machine.

#### Workloads

To run the `frontend` container, use:

```shell
podman run --name frontend -d --rm -p 127.0.0.1:7070:8080 quay.io/skupper/hello-world-frontend --backend http://host.containers.internal:8080
```

And to run the `backend` container, use:

```shell
podman run --name backend -d --rm -p 127.0.0.1:9090:8080 quay.io/skupper/hello-world-backend
```

#### West

The `west` site will be defined using the following CRs:

##### Site
```yaml
apiVersion: skupper.io/v1alpha1
kind: Site
metadata:
  name: west
```

##### Listener

```yaml
apiVersion: skupper.io/v1alpha1
kind: Listener
metadata:
  name: backend
spec:
  host: 0.0.0.0
  port: 8080
  routingKey: backend-8080
```

##### RouterAccess

```yaml
apiVersion: skupper.io/v1alpha1
kind: RouterAccess
metadata:
  name: go-west
spec:
  roles:
    - port: 55671
      name: inter-router
    - port: 45671
      name: edge
  bindHost: 127.0.0.1
```

Now that the `west` site has been created, copy the generated token into
the `east` site local directory. Example:

```shell
cp ${HOME}/.local/share/skupper/namespaces/default/runtime/token/link-go-west.yaml ./east/links/link-go-west.yaml
```

#### East

The `east` site will be defined using the following CRs:

**Site**
```yaml
apiVersion: skupper.io/v1alpha1
kind: Site
metadata:
  name: east
```

**Connector**
```yaml
---
apiVersion: skupper.io/v1alpha1
kind: Connector
metadata:
  name: backend
spec:
  host: 127.0.0.1
  port: 9090
  routingKey: backend-8080
```

**Link**
```yaml
---
apiVersion: v1
data:
  ca.crt: ___redacted___
  connect.json: ___redacted___
  tls.crt: ___redacted___
  tls.key: ___redacted___
kind: Secret
metadata:
  name: link-go-west
---
apiVersion: skupper.io/v1alpha1
kind: Link
metadata:
  name: link-go-west
spec:
  cost: 1
  endpoints:
  - host: 127.0.0.1
    name: inter-router
    port: "55671"
  - host: 127.0.0.1
    name: edge
    port: "45671"
  tlsCredentials: link-go-west
```

### Testing the scenario

Once both sites have been initialized, open **http://127.0.0.1:7070**
in your browser and it should work.
