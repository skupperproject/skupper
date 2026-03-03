package site

import (
	"strconv"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

// multiAddressTcpListenerName returns the tcpListener name for a MultiKeyListener.
// The format "multiAddress/<name>" avoids collisions with regular Listener resources.
func multiAddressTcpListenerName(name string) string {
	return "multiAddress/" + name
}

// listenerAddressName returns the listenerAddress name for a routing key.
// The format "<mkl-name>/<address>" provides clear association with the parent listener.
func listenerAddressName(mklName, address string) string {
	return mklName + "/" + address
}

// UpdateBridgeConfigForMultiKeyListener creates the tcpListener and listenerAddress
// entities needed to implement a MultiKeyListener with priority strategy.
func UpdateBridgeConfigForMultiKeyListener(siteId string, mkl *skupperv2alpha1.MultiKeyListener, config *qdr.BridgeConfig) {
	UpdateBridgeConfigForMultiKeyListenerWithHostAndPort(siteId, mkl, mkl.Spec.Host, mkl.Spec.Port, config)
}

// UpdateBridgeConfigForMultiKeyListenerWithHostAndPort creates the tcpListener and
// listenerAddress entities with the specified host and port.
func UpdateBridgeConfigForMultiKeyListenerWithHostAndPort(siteId string, mkl *skupperv2alpha1.MultiKeyListener, host string, port int, config *qdr.BridgeConfig) {
	name := mkl.Name
	tcpListenerName := multiAddressTcpListenerName(name)

	config.AddTcpListener(qdr.TcpEndpoint{
		Name:                 tcpListenerName,
		SiteId:               siteId,
		Host:                 host,
		Port:                 strconv.Itoa(port),
		SslProfile:           mkl.Spec.TlsCredentials,
		MultiAddressStrategy: "priority",
		AuthenticatePeer:     mkl.Spec.RequireClientCert,
	})

	// Create listenerAddress entities for each routing key in the strategy
	if mkl.Spec.Strategy.Priority != nil {
		numKeys := len(mkl.Spec.Strategy.Priority.RoutingKeys)
		for i, routingKey := range mkl.Spec.Strategy.Priority.RoutingKeys {
			laName := listenerAddressName(name, routingKey)
			config.AddListenerAddress(qdr.ListenerAddress{
				Name:     laName,
				Address:  routingKey,
				Value:    numKeys - 1 - i, // higher value = higher priority
				Listener: tcpListenerName,
			})
		}
	}
}

// RemoveBridgeConfigForMultiKeyListener removes the tcpListener and listenerAddress
// entities for a MultiKeyListener. This function removes all listenerAddress entities
// with the given listener name as their reference.
func RemoveBridgeConfigForMultiKeyListener(name string, config *qdr.BridgeConfig) {
	tcpListenerName := multiAddressTcpListenerName(name)
	// Remove all listenerAddresses that reference this listener
	for laName, la := range config.ListenerAddresses {
		if la.Listener == tcpListenerName {
			config.RemoveListenerAddress(laName)
		}
	}
	// Remove the tcpListener
	config.RemoveTcpListener(tcpListenerName)
}
