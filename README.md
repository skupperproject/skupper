# Skupper

Skupper enables cloud communication by enabling you to create a Virtual Application Network.

This upper layer network decouples addressing from the underlying network infrastructure, providing a secure and simple alternative to VPNs.

You can use Skupper to create a network from namespaces inn one or more Kubernetes clusters as described in the [Getting Started](https://skupper.io/start/index.html).
This guide describes a simple network, however there are no restrictions on the topology created which can include redundant paths. 
You can also create more complex (full mesh) networks, Skupper then supports anycast and multicast communication.

Skupper does not require any special privileges, that is, you do not require the `cluster-admin` role to create networks.

# Useful Links
Using Skupper

* [Getting Started](https://skupper.io/start/index.html)
* [Examples](https://skupper.io/examples/index.html)
* [Documentation](https://skupper.io/docs/index.html)


Developing Skupper

* [Community](https://skupper.io/community/index.html)
* [Site controller](cmd/site-controller/README.md)
* [CLI](cmd/skupper/README.md) (This replaces the [Skupper CLI repo](https://github.com/skupperproject/skupper-cli))
* [Console](/skupperproject/gilligan)

# Licensing
Skupper uses the [Apache QPID Dispatch Router](https://github.com/apache/qpid-dispatch) project and is released under the same [Apache License 2.0](https://github.com/skupperproject/skupper/blob/master/LICENSE).
