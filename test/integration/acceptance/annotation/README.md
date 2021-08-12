# Test how Skupper handles annotated resources

# Description
The "TestAnnotatedResources" test validates how Skupper reacts when supported
resources (Service and Deployment) are updated with Skupper related
annotations.

It uses an nginx server, as the target component for the exposed services,
created by Skupper, from an annotated Deployment and two annotated services
(one uses a target address while the other does not).

Each test case, from the test table, runs a different scenario, performing
modifications to the initially annotated resources.

The final validation, for all test cases, is the same.

They all validate:

* From http://skupper-controller:8080/DATA:
  * The number of connected sites
  * The final list of services 
    * Number of services
    * The exposed service list and their respective protocol
* Each exposed service should be accessible
    * Return code is 200
    * Response body is not empty
    
# Pre-requisites

* At least one cluster must be available, through:
  * KUBECONFIG environment variable (defaults to: `${USER}/.kube/config`)
  * Or specifying one or two clusters through:
    * `--kubeconfig`
    * `--edgekubeconfig`
* Test will create two namespaces
  * `annotation-1`
  * `annotation-2`
  * ***Note:** If using 2 clusters, each one will be created in a distinct cluster*

# Setup and TearDown

The Setup creates the namespaces, deploys the initial set of
resources (Deployment and Services) and creates the Skupper
network between the two namespaces (or clusters). This happens
only once for all test cases.

The TearDown is executed when test finishes or is interrupted,
and it deletes the created namespaces (if namespace creation fails,
it does not delete anything).

# Test steps and validations (common for all test cases)

1. Run the modification function (if any provided)
1. Retrieve services list from http://skupper-controller:8080/DATA
1. Validate number of sites
1. Validate exposed list of services and their protocols
1. Communicate with all exposed services from both clusters
   1. Expects HTTP status code to be 200
   1. Response body cannot be empty

# Test cases

## services-pre-annotated

First iteration to run. It creates an nginx deployment and two services on
each cluster/ns. They are created with Skupper annotations, before Skupper
is deployed to the clusters, to validate if Skupper is detecting the existing
services and exposing them as per the annotations.

Here are the annotations on each:

* **Cluster1**
  * nginx (Deployment)
    * skupper.io/proxy = tcp
    * skupper.io/address = nginx-1-dep-web
  * nginx-1-svc-exp-notarget (Service)
    * skupper.io/proxy = tcp
  * nginx-1-svc-target (Service)
    * skupper.io/proxy = http
    * skupper.io/address = nginx-1-svc-exp-target
  
* **Cluster2**
  * nginx (Deployment)
    * skupper.io/proxy = tcp
    * skupper.io/address = nginx-2-dep-web
  * nginx-2-svc-exp-notarget (Service)
    * skupper.io/proxy = tcp
  * nginx-2-svc-target (Service)
    * skupper.io/proxy = http
    * skupper.io/address = nginx-2-svc-exp-target

## services-protocol-switch

Switches the protocols for all exposed resources. If resource is using
TCP, it will set the protocol to HTTP and vice-versa.

## services-annotation-removed

Removes all Skupper annotations from previously annotated resources.

## services-annotated

Adds the annotations back to the initial resources (now with Skupper already
running) and validate that it has been restored to the initial state.
