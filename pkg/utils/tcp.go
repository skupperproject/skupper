package utils

import (
	"fmt"
	"net"
	"strconv"
)

func TcpPortInUse(host string, port int) bool {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return true
	}
	if listener != nil {
		_ = listener.Close()
	}
	return false
}

func TcpPortNextFree(startPort int) (int, error) {
	for port := startPort; port <= 65535; port++ {
		if !TcpPortInUse("", port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found")
}
