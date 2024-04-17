# Skupper CLI spec

## Hello World, briefly

~~~ console
# Get the CLI

$ curl https://skupper.io/install.sh | curl

# West

$ export KUBECONFIG=~/.kube/config-west
$ kubectl apply -f https://skupper.io/install.yaml
$ kubectl create deployment frontend --image quay.io/skupper/hello-world-frontend

$ skupper site create --ingress loadbalancer
$ skupper token create ~/token.yaml
$ skupper listener create backend --host backend --port 8080

# East

$ export KUBECONFIG=~/.kube/config-east
$ kubectl apply -f https://skupper.io/install.yaml
$ kubectl create deployment backend --image quay.io/skupper/hello-world-backend --replicas 3

$ skupper site create
$ skupper link create ~/token.yaml
$ skupper connector create backend --workload deployment/backend --port 8080
~~~

## Philosophy

The Skupper CLI in version 2 is deliberately a light layer on top of
the standard Skupper customer resources.  Its job in short is to
render the YAML resource, submit it to the platform, and wait for the
desired result.

The `create` and `set` operations in particular are meant to provide a
convenient and CLI-conventional interface, as an alternative to
writing YAML by hand.

In general, the operations block until the user's desired outcome is
achieved.  If the user wants something asynchronous, they can use
`kubectl` commands.

## Example site operations

~~~ console
$ skupper site create --name west --ingress loadbalancer
Waiting for status...
Waiting for ingress...
Site "west" is ready

$ skupper site get
NAME   STATUS   INGRESS
west   Ready    loadbalancer

$ skupper site get -o yaml
apiVersion: v2alpha1
kind: Site
[...]

$ skupper site set --ingress route
Waiting for ingress...
Site "west" is ready

$ skupper site delete
Waiting for site "west" to delete...
Site "west" is deleted
~~~

## Example token operations

~~~ console
$ skupper token create token.yaml
Token file created at token.yaml
The token expires after 1 use or after 15 minutes
~~~

## Example link operations

~~~ console
$ skupper link create token.yaml
Waiting for link "west" to become active...
Link "west" is active
You can now delete token.yaml

$ skupper link get
NAME    STATUS   COST
south   Error    10
west    Active   1

$ skupper link get west
NAME   STATUS   COST
west   Active   1

$ skupper link get west -o yaml
apiVersion: v2alpha1
kind: Link
[...]

$ skupper link delete west
Waiting for link "west" to delete...
Link "west" is deleted
~~~

## Example listener operations

~~~ console
$ skupper listener create database --host database --port 5432
Waiting for listener...
Listener "database" is ready

$ skupper listener get
NAME       ROUTING-KEY   HOST       PORT
payments   payments      payments   8080
database   database      database   5432

$ skupper listener get database -o yaml
apiVersion: v2alpha1
kind: Listener
[...]

$ skupper listener set database --port 5431
Waiting for listener...
Listener "database" is ready

$ skupper listener delete database
Listener "database" is deleted
~~~

## Example connector operations

~~~ console
$ skupper connector create database --workload deployment/database --port 5432
Waiting for connector...
Connector "database" is ready

$ skupper connector get
NAME         ROUTING-KEY   SELECTOR       HOST   PORT
database     database      app=database   -      5432

$ skupper connector get database -o yaml
apiVersion: v2alpha1
kind: Connector
[...]

$ skupper connector set database --port 5431
Waiting for connector...
Connector "database" is ready

$ skupper connector delete database
Connector "database" is deleted
~~~

## Skupper resource commands

These are the core Skupper commands.  They are not the only commands,
however.  Additional commands will be added to the spec in the future.
For instance, we will have commands for debugging, getting versions,
and for installing Skupper on a platform.

### `skupper <resource-type>`

Print help text for the operations of this resource type.

### `skupper <resource-type> create [options]`

Create a resource.

Resource options are set using one or more `--some-key some-value`
command line options.  YAML resource options in camel case (`someKey`)
are exposed as hyphenated names (`some-key`) when used as command line
options.

#### Positional arguments

`token create <token-file>` - `token create` takes one positional
parameter, the file in which to create the token.

`link create <token-file>` - `link create` takes one positional
parameter, the file containing the token.

`listener create <routing-key>` - `listener create` takes one
 positional parameter, the routing key used to match the listener to
 connectors.

On Kubernetes, `listener create` also requires a `--host <host>`
option and a `--port <port>` option.

`connector create <routing-key>` - `connector create` takes one
positional parameter, the the routing key used to match the connector
to listeners.

On Kubernetes, `connector create` also requires a `--selector
<selector>`, `--host <host>`, or `--workload <type>/<name>` option and
a `--port` option.

#### Blocking

`site create` blocks until the site is ready, including ingress if
configured.

`link create` blocks until the link is active.

`listener create` blocks until the router listener is ready to accept
connections.

`connector create` blocks until the router connector is ready to make
connections.

### `skupper <resource-type> delete <resource-name>`

Delete a resource.

Since site is a singleton, the resource name argument is not required
for site deletion.

#### Blocking

The delete operation blocks until the resource is removed.

### `skupper <resource-type> get [<resource-name>]`

This works just like `kubectl get <type>/<name>`.  In the first
iteration of the CLI, I think we should just delegate to kubectl.

`get` without a qualifying resource name argument enumerates all the
resources of this type.  (Same as `kubectl get`.)

`get` with a resource name and `--output yaml` gives you the full
resource YAML.  (Same as `kubectl get`.)

### `skupper <resource-type> set <resource-name> [options]`

Set resource options.

`set` is very similar to `create`.  Instead of creating a new resource
from defaults, it updates an existing one.

This takes one or more long options (`--name foo`).  They are the same
options as those used on create.

Since site is a singleton, the resource name argument is not required
for setting site options.

#### Blocking

`set` blocks with the same rules expressed for `create`.

### `skupper <resource-type> <special-operation>`

An operation specific to a particular resource type.

For example, if in the future we have the ability to join a network
using an invitation, we might offer `skupper site join <invitation>`.

<!-- ## Skupper installation commands -->

<!-- ### `skupper install` -->

<!-- Kube: Equivalent to `kubectl apply -f https://skupper.io/install.yaml` -->

<!-- Blocks until: The Skupper controller is ready -->

<!-- Consider: Should this fall back to some static version of the install -->
<!-- YAML burned into the CLI?  For disconnected cases. -->

<!-- ### `skupper uninstall` -->

<!-- Kube: Equivalent to `kubectl delete -f https://skupper.io/install.yaml` -->

<!-- Blocks until: The Skupper resources are removed -->

## Thoughts

It's important that the Hello World steps work as scripted with no
sleeps or additional logic to wait on conditions.  That's why I have
the notes about how operations block.

By default, no ingress.  Overall, it seems a bit better to require
people specify when they want it.

## Guidelines

* Commands block until the user's desired state is achieved.
* No blocking on user input.
