package utils

import (
	"fmt"
	"net"
	"os"
)

func GetSansByDefault() ([]string, error) {
	sans := []string{"0.0.0.0", "::"}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	sans = append(sans, hostname)

	// Get all local IP addresses
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addresses {

		if ipnet, ok := addr.(*net.IPNet); ok {
			// Add only non-loopback IPv4 and IPv6 addresses
			if !ipnet.IP.IsLoopback() {
				sans = append(sans, ipnet.IP.String())
			}
		}
	}

	return sans, nil
}

func AllocateRouterAccessPorts() (interRouterPort int, edgePort int, err error) {

	defaultInterRouterPort := 55671
	defaultEdgePort := 45671

	interRouterPort, err = TcpPortNextFree(defaultInterRouterPort)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate inter-router port: %w", err)
	}

	edgePort, err = TcpPortNextFree(defaultEdgePort)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate edge port: %w", err)
	}

	if edgePort == interRouterPort {
		edgePort, err = TcpPortNextFree(edgePort + 1)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to allocate alternative edge port: %w", err)
		}
	}

	return interRouterPort, edgePort, nil
}
