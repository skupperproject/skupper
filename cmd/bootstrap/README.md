# Bootstrapping non-kubernetes sites

In this current phase of the Skupper V2 implementation, non-kubernetes sites
can be bootstrapped using a locally built binary, that can be produced by running
`make build-bootstrap`, or using the quay.io/skupper/bootstrap:v2-latest
container image by calling `./cmd/bootstrap/bootstrap.sh` with the appropriate
flags.

It is important to mention that the bootstrap procedure executed by the provided
`bootstrap` binary, will be incorporated into the Skupper CLI shortly.

Non-kubernetes sites can be created using the standard V2 site declaration
approach, which is based on the new set of Custom Resource Definitions (CRDs).

The same CRDs used to create a V2 Skupper site on a Kubernetes cluster can be
used to create non-kubernetes sites. Some particular options and fields are
valid or required only on specific platforms, but the overall structure is the
same.

Differently than the current Kubernetes implementation, non-kubernetes sites
are static. Meaning that you must have all your Custom Resources (CRs) ready
at the time your site is bootstrapped and if something needs to change, you
will need to bootstrap it over. If you do that, the Certificate Authorities (CAs)
are preserved, so if there is any eventual existing incoming link, it should
be able to reconnect.

## Supported platforms

You can specify the target platform to be bootstrapped by exporting
the `SKUPPER_PLATFORM` environment variable, using one of the following
allowed values:

* podman
* docker
* systemd

### Container engine platforms

When `podman` or `docker` platform is used, then the bootstrap procedure will
require that the respective container engine endpoint is available. The default
unix socket will be used based on the current user and platform selected.

### Systemd

The `systemd` platform actually requires that you have a local installation of
the `skupper-router` (`skrouterd` binary must be available in your PATH).

## Bootstrap usage

### Bootstrap command and flags

```shell
Skupper bootstrap

Bootstraps a nonkube Skupper site base on the provided flags.

When the path (-p) flag is provided, it will be used as the source
directory containing the Skupper custom resources to be processed,
generating a local Skupper site using the "default" namespace, unless
a namespace is set in the custom resources, or if the namespace (-n)
flag is provided.

A namespace is just a directory in the file system where all site specific
files are stored, like certificates, configurations, the original sources
(original custom resources used to bootstrap the nonkube site) and
the runtime files generated during initialization.

In case the path (-p) flag is omitted, Skupper will try to process
custom resources stored at the sources directory of the default namespace,
or from the namespace provided through the namespace (-n) flag.

If the respective namespace already exists and you want to bootstrap it
over, you must provide the force (-f) flag. When you do that, the existing
Certificate Authorities (CAs) are preserved, so eventual existing incoming
links should be able to reconnect.

To produce a bundle, instead of rendering a site, the bundle strategy (-b)
flag must be set to "bundle" or "tarball".

Usage:
  bootstrap [options...]

Flags:
  -b string
    	The bundle strategy to be produced: bundle or tarball
  -f	Forces to overwrite an existing namespace
  -n string
    	The target namespace used for installation
  -p string
    	Custom resources location on the file system
  -v	Report the version of the Skupper bootstrap command
```

### Bootstrapping and removing non-kubernetes sites

Place all your Custom Resources (CRs) on a local directory.
You must always provide a `Site` (CR), plus some other resource
that makes your running site meaningful, like Listeners and/or Connectors.

In case you want your local site to accept incoming links from other sites,
at present, you have to explicitly provide a `RouterAccess` (CR).

If you want your non-kubernetes site to establish links to other sites, make
sure you also provide `AccessToken` or `Link` (CRs).

If the CRs have no namespace set, Skupper assumes "default" as the namespace
to be used, otherwise it will use the namespace defined in the CRs, unless you
force a specific namespace to be used through the `-n` flag.

***NOTES:** A V2 representation of the **"Hello world example"** is available below
to provide initial guidance.*

Now that you have all your CRs placed on a local directory, just run:

#### Bootstrapping

```shell
# To bootstrap your site using the binary
export SKUPPER_PLATFORM=podman
./bootstrap -p ./
```

You can also use `-n=<name>` to override the namespace specified in the CRs.

Alternatively you can also run the `bootstrap.sh` script:

```shell
# To bootstrap your site using the shell script
export SKUPPER_PLATFORM=podman
./cmd/bootstrap/bootstrap.sh -p ./
```

Similarly to the binary, you can also use `-n <namespace>` to override the namespace
defined in the source CRs.

Remember that you can set the `SKUPPER_PLATFORM` environment variable to
any of the platforms mentioned earlier.

If you do not want to run an actual site from your CRs, but instead, you just
need to produce a bundle that can be installed somewhere else, then you can
use `bundle` or `tarball` as the bundle strategy (-b flag).

The `bundle` strategy will produce a self-extracting shell archive that can
be executed to install your local non-kubernetes site.

If you are not comfortable executing the produced script, you can also choose
`tarball` as the bundle strategy. It produces a tar ball that contains an
`install.sh` script, that basically performs the same procedure of the
self-extracting `bundle`.

After the bootstrap procedure is completed, it will provide you some relevant
information like:

* Location where static links have been defined (when a `RouterAccess` is provided)
* Location where the site bundle has been saved (if bundle strategy flag `-b` set)
* Namespace, Site name, Platform, Version and path to the sources

#### Site bundles

When a site bundle is produced, the bootstrap procedure will point you
to the location where it has been saved.

##### Bundle installation usage

The bundle installation script accepts the following flags:

```
-h               help
-p <platform>    podman, docker, systemd
-n <namespace>   if not provided, the namespace defined in the bundle is used (if none, default is used)
-x               remove site and namespace
-d <directory>   dump static links into the provided directory 
```

#### Removing

To remove your site, you can run a local script, that takes the namespace
as an argument. The platform is identified by the script so you don't need
to export it.

If the namespace is omitted, the "default" namespace is used. 

```shell
./cmd/bootstrap/remove.sh [namespace]
```

## Example

Here is a very basic demonstration on how you can run the `Hello World` example
locally using just non-kubernetes sites.

The example assumes you are running both `west` and `east` sites on your
machine. So if you want to run them on different machines, make sure to adjust
IP addresses properly.

### Hello world

This example basically runs a **frontend** application which depends on
a **backend** service that is not initially accessible. The access will
be provided by the Skupper Site running on the `east` namespace.

To simulate it, the frontend application will be executed using podman.
It exposes port 7070 in the loopback interface of the host machine,
and must tell it to expect that the backend service will be available
at `http://host.containers.internal:8080`, which from inside the container,
means the host machine at port 8080.

The backend also runs on the host machine with podman, and it
exposes port 9090 to the loopback interface of the host machine.
So after both `frontend` and `backend` containers are running,
they won't be able to communicate.

We will use two Skupper sites to resolve that. On the `west` site, Skupper
will expose a `Listener` (CR), bound to port `8080` of the host machine.

On the `east` site, we will have a `Connector` (CR) that targets localhost
at port 9090, in the host machine.

The ideal scenario would be to run each component on different machines,
where Skupper could add real value.
But just with the purpose of making it simple to be executed locally,
this example has been designed so that you can run everything using a
single machine.

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

##### Preparing the CRs

The `west` site will be defined using the following CRs:

##### Site
```yaml
apiVersion: skupper.io/v1alpha1
kind: Site
metadata:
  name: west
  namespace: west
```

##### Listener

```yaml
apiVersion: skupper.io/v1alpha1
kind: Listener
metadata:
  name: backend
  namespace: west
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
  namespace: west
spec:
  roles:
    - port: 55671
      name: inter-router
    - port: 45671
      name: edge
  bindHost: 127.0.0.1
```

##### Bootstrap the west site

Considering all your CRs have been saved to a directory named `west`, use:

```shell
./bootstrap -p ./west
```

The CRs are defined using the `west` namespace, so it will be used by default.
If the namespace was empty, skupper would try to set it to `default`.

Now that the `west` site has been created, copy the generated link into
the `east` site local directory. Example:

```shell
cp ${HOME}/.local/share/skupper/namespaces/west/runtime/link/link-go-west.yaml ./east/link-go-west.yaml
```

Make sure to update the namespace on the generated `link` that will be used
in the `east` site. To do that, run:

```shell
sed -i 's/namespace: west/namespace: east/g' ./east/link-go-west.yaml
```

#### East

The `east` site will be defined using the following CRs:

**Site**
```yaml
apiVersion: skupper.io/v1alpha1
kind: Site
metadata:
  name: east
  namespace: east
```

**Connector**
```yaml
---
apiVersion: skupper.io/v1alpha1
kind: Connector
metadata:
  name: backend
  namespace: east
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
  namespace: east
---
apiVersion: skupper.io/v1alpha1
kind: Link
metadata:
  name: link-go-west
  namespace: east
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

##### Bootstrap the east site

Considering all your CRs have been saved to a directory named `east`, use:

```shell
./bootstrap -p ./east
```

### Testing the scenario

Once both sites have been initialized, open **http://127.0.0.1:7070**
in your browser and it should work.

### Updating an existing installation

Suppose modifications have been made to the `west` site CRs, directly at the
namespace directory (i.e: ${HOME}/.local/share/skupper/namespaces/west/sources).

To re-initialize the west site, run:

```shell
bootstrap -n west -f
```

The command above will reprocess all source CRs from the namespace path and
restart the related components. The Certificate Authorities (CAs) are preserved,
therefore eventual incoming links must still work.

In case the CRs are located somewhere else, then a path must also be specified, as
in the example below:

```shell
bootstrap -n west -p <path> -f
```

### Cleanup

To remove both namespaces, run:

```shell
./cmd/bootstrap/remove.sh west
./cmd/bootstrap/remove.sh east
```

### Producing site bundles

As an alternative, you can also produce site bundles to try this example.

#### Creating the west bundle

```shell
$ bootstrap -p ./west/ -b bundle
Skupper nonkube bootstrap (version: main-release-161-g7c5100a2-modified)
Site "west" has been created (as a distributable bundle)
Installation bundle available at: /home/user/.local/share/skupper/bundles/skupper-install-west.sh
Default namespace: west
Default platform: docker
```

#### Extracting the static link

To extract the link, run:

```shell
/home/user/.local/share/skupper/bundles/skupper-install-west.sh -d /tmp
Static links for site "west" have been saved into /tmp/west
```

Copy the static Link to the `east` site definition and update the namespace.

```shell
cp /tmp/west/link-go-west.yaml ./east/link-go-west.yaml
sed -i 's/namespace: west/namespace: east/g' ./east/link-go-west.yaml
```

#### Creating the east bundle

```shell
$ bootstrap -p ./east -b bundle
Skupper nonkube bootstrap (version: main-release-161-g7c5100a2-modified)
Site "east" has been created (as a distributable bundle)
Installation bundle available at: /home/user/.local/share/skupper/bundles/skupper-install-east.sh
Default namespace: east
Default platform: podman
```

#### Installing both bundles

```shell
/home/user/.local/share/skupper/bundles/skupper-install-west.sh
/home/user/.local/share/skupper/bundles/skupper-install-east.sh
```

#### Cleanup bundle installation

```shell
/home/user/.local/share/skupper/bundles/skupper-install-west.sh -x
/home/user/.local/share/skupper/bundles/skupper-install-east.sh -x
```
