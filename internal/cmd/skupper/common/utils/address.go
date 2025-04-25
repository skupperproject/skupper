package utils

import (
	"net"
	"os"
)

func GetSansByDefault() ([]string, error) {
	sans := []string{}

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
