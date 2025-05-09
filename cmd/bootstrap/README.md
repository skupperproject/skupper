# Bootstrapping non-kubernetes sites

In this current phase of the Skupper V2 implementation, non-kubernetes sites
can be bootstrapped using the Skupper CLI, using the quay.io/skupper/skupper-cli:v2-dev
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
* linux

### Container engine platforms

When `podman` or `docker` platform is used, then the bootstrap procedure will
require that the respective container engine endpoint is available. The default
unix socket will be used based on the current user and platform selected.

### Linux

The `linux` platform actually requires that you have a local installation of
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

Namespaces are stored under ${XDG_DATA_HOME}/skupper/namespaces
for regular users when XDG_DATA_HOME environment variable is set, or under
${HOME}/.local/share/skupper/namespaces when it is not set.
As the root user, namespaces are stored under: /var/lib/skupper/namespaces.

Skupper will process custom resources stored at the input/resources directory 
of the default namespace, or from the namespace provided through the namespace (-n) flag.


To produce a bundle, instead of rendering a site, the bundle strategy (-b)
flag must be set to "bundle" or "tarball".

This action needs the path (-p) flag is provided, that it will be used as the source
directory containing the Skupper custom resources to be processed,
generating a local Skupper site using the "default" namespace, unless
a namespace is set in the custom resources, or if the namespace (-n)
flag is provided.


Usage:
  bootstrap [options...]

Flags:
  -b string
    	The bundle strategy to be produced: bundle or tarball
  -n string
    	The target namespace used for installation
  -p string
    	Custom resources location on the file system for the bundle
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
# To bootstrap your site using Skupper
export SKUPPER_PLATFORM=podman
skupper system start
```

You can also use `-n=<name>` to override the namespace specified in the CRs.

Alternatively you can also run the `bootstrap.sh` script:

```shell
# To bootstrap your site using the shell script
export SKUPPER_PLATFORM=podman
./cmd/bootstrap/bootstrap.sh 
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
* Namespace, Site name, Platform, Version and path to the resources

#### Site bundles

When a site bundle is produced, the bootstrap procedure will point you
to the location where it has been saved.

##### Bundle installation usage

The bundle installation script accepts the following flags:

```
-h               help
-p <platform>    podman, docker, linux
-n <namespace>   if not provided, the namespace defined in the bundle is used (if none, default is used)
-x               remove site and namespace
-d <directory>   dump static links into the provided directory 
```

#### Removing

To remove your site, you can run  the `system stop` command, providing a namespace as a flag.
If the namespace is omitted, the "default" namespace is used. 

```shell
skupper system stop -n [namespace]
```
Or you can run a local script, that takes the namespace
as an argument. The platform is identified by the script so you don't need
to export it.

```shell
./cmd/bootstrap/remove.sh [namespace]
```

## Using custom certificates

Users can provide their own certificates to be used when initializing a local site,
when preparing a site bundle to be installed somewhere else and even during a site
bundle installation time.

More about user provided certificates can be found [here](PROVIDED_CERTIFICATES.md).

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
apiVersion: skupper.io/v2alpha1
kind: Site
metadata:
  name: west
```

##### Listener

```yaml
apiVersion: skupper.io/v2alpha1
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
apiVersion: skupper.io/v2alpha1
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

Equivalent in CLI commands: 
```
skupper site create west --enable-link-access -n west
skupper listener create backend 8080 -n west
```
Note the CLI takes care of the creation of the RouterAccess resource.


##### Bootstrap the west site

Considering all your CRs have been saved to the namespace named `west`, use:

```shell
./cmd/bootstrap/bootstrap.sh -n west
```
or 

```shell
skupper system start -n west
```

The CRs are defined without a namespace, that is why we have the `-n west` flag,
to indicate we want this site to run under the `west` namespace.
Otherwise, the `default` namespace would be used.

Now that the `west` site has been created, copy the generated link into
the `east` site local directory. Example:

```shell
cp ${HOME}/.local/share/skupper/namespaces/west/runtime/links/link-go-west-127.0.0.1.yaml ./east/link-go-west.yaml
```

#### East

The `east` site will be defined using the following CRs:

**Site**
```yaml
apiVersion: skupper.io/v2alpha1
kind: Site
metadata:
  name: east
```

**Connector**
```yaml
---
apiVersion: skupper.io/v2alpha1
kind: Connector
metadata:
  name: backend
spec:
  host: 127.0.0.1
  port: 9090
  routingKey: backend-8080
```

Equivalent in CLI commands:
```
skupper site create east -n east
skupper connector create backend 9090 --host 127.0.0.1 --routing-key backend -n east
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
apiVersion: skupper.io/v2alpha1
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

##### Bootstrap the east site

Considering all your CRs have been saved to a namespace named `east`, use:

```shell
./cmd/bootstrap/bootstrap.sh -n east
```

or 

```shell
skupper system start -n east
```

### Testing the scenario

Once both sites have been initialized, open **http://127.0.0.1:7070**
in your browser and it should work.

### Updating an existing installation

Suppose modifications have been made to the `west` site CRs, directly at the
namespace directory (i.e: ${HOME}/.local/share/skupper/namespaces/west/input/resources).

To re-initialize the west site, run:

```shell
system reload -n west 
```

The command above will reprocess all source CRs from the namespace path and
restart the related components. The Certificate Authorities (CAs) are preserved,
therefore eventual incoming links must still work.


### Cleanup

To remove both namespaces, run:

```shell
skupper system stop -n west
skupper system stop -n east
```

### Producing site bundles

As an alternative, you can also produce site bundles to try this example.

#### Creating the west bundle

```shell
$ ./cmd/bootstrap/bootstrap.sh -p ./west/ -b bundle
Skupper nonkube bootstrap (version: main-release-161-g7c5100a2-modified)
Site "west" has been created (as a distributable bundle)
Installation bundle available at: /home/user/.local/share/skupper/bundles/skupper-install-west.sh
Default namespace: default
Default platform: podman
```
or 

```shell
skupper system generate-bundle my-bundle --input ./west  --type shell-script                                       
2024/11/18 12:17:56 updating listener /backend...
Site "west" has been created (as a distributable bundle)
Installation bundle available at: {HOME}/.local/share/skupper/bundles/my-bundle.sh
Default namespace: default
Default platform: podman
```

#### Extracting the static links

To extract the links, run:

```shell
/home/user/.local/share/skupper/bundles/skupper-install-west.sh -d /tmp
Static links for site "west" have been saved into /tmp/west
```

Copy the static link to the `east` site definition.

```shell
cp /tmp/west/link-go-west-127.0.0.1.yaml ./east/link-go-west.yaml
```

#### Creating the east bundle

```shell
$./cmd/bootstrap/bootstrap.sh -p ./east -b bundle
Skupper nonkube bootstrap (version: main-release-161-g7c5100a2-modified)
Site "east" has been created (as a distributable bundle)
Installation bundle available at: /home/user/.local/share/skupper/bundles/skupper-install-east.sh
Default namespace: default
Default platform: podman
```

#### Installing both bundles

```shell
/home/user/.local/share/skupper/bundles/skupper-install-west.sh -n west
/home/user/.local/share/skupper/bundles/skupper-install-east.sh -n east
```

#### Cleanup bundle installation

```shell
/home/user/.local/share/skupper/bundles/skupper-install-west.sh -x
/home/user/.local/share/skupper/bundles/skupper-install-east.sh -x
```
