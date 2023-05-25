package main

import (
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/pkg/ports"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func portAsInt(port string) int {
	result, _ := strconv.Atoi(port)
	return result
}

func getPortAllocations(ports *ports.FreePorts, bridges *qdr.BridgeConfig) map[string][]int {
	allocations := map[string][]int{}
	addPort := func(address string, port int) {
		if curPorts, found := allocations[address]; !found {
			allocations[address] = []int{port}
		} else {
			curPorts = append(curPorts, port)
			allocations[address] = curPorts
		}
	}
	if bridges != nil {
		for _, b := range bridges.HttpConnectors {
			port := portAsInt(b.Port)
			ports.InUse(port)
		}
		for _, b := range bridges.HttpListeners {
			address := strings.Split(b.Address, ":")[0]
			port := portAsInt(b.Port)
			addPort(address, port)
			ports.InUse(port)
		}
		for _, b := range bridges.TcpConnectors {
			port := portAsInt(b.Port)
			ports.InUse(port)
		}
		for _, b := range bridges.TcpListeners {
			address := strings.Split(b.Address, ":")[0]
			port := portAsInt(b.Port)
			addPort(address, port)
			ports.InUse(port)
		}
	}
	return allocations
}
