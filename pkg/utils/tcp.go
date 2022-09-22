package utils

import (
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
