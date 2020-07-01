# Skupper

Skupper enables cloud communication by enabling you to create a Virtual Application Network.

This application layer network decouples addressing from the underlying network infrastructure.
One example usage is to provide a secure and simple alternative to a VPN.

You can use Skupper to create a network from namespaces inn one or more Kubernetes clusters as described in the [Getting Started](https://skupper.io/start/index.html).
This guide describes a simple network, however there are no restrictions on the topology created which can include redundant paths.

You can also create more complex networks, for example, a full mesh network.

Skupper supports [anycast](https://en.wikipedia.org/wiki/Anycast) and [multicast](https://en.wikipedia.org/wiki/Multicast) communication using the application layer network (VAN), allowing you to configure your topology to match business requirements.

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
