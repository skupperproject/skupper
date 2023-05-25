package qdr

import (
	"log"
	"strconv"

	"github.com/skupperproject/skupper/pkg/ports"
)

type PortMapping struct {
	mappings map[string]int
	pool     *ports.FreePorts
}

func (p *PortMapping) GetPortForAddress(address string) (int, error) {
	if existing, ok := p.mappings[address]; ok {
		return existing, nil
	}
	allocated, err := p.pool.NextFreePort()
	if err != nil {
		return 0, err
	}
	p.mappings[address] = allocated
	return allocated, err
}

func (p *PortMapping) ReleasePortForAddress(address string) {
	if existing, ok := p.mappings[address]; ok {
		p.pool.Release(existing)
		delete(p.mappings, address)
	}
}

func (p *PortMapping) recovered(address string, portstr string) {
	port, err := strconv.Atoi(portstr)
	if err != nil {
		log.Printf("Failed to convert port %q to int: %s", portstr, err)
		return
	}
	p.pool.InUse(port)
	p.mappings[address] = port
	if existing, ok := p.mappings[address]; ok {
		p.pool.Release(existing)
		delete(p.mappings, address)
	}
}

func RecoverPortMapping(config *RouterConfig) *PortMapping {
	mapping := &PortMapping{
		mappings: map[string]int{},
		pool:     ports.NewFreePorts(),
	}
	for _, listener := range config.Listeners {
		mapping.pool.InUse(int(listener.Port))
	}

	for _, listener := range config.Bridges.TcpListeners {
		mapping.recovered(listener.Address, listener.Port)
	}
	for _, listener := range config.Bridges.HttpListeners {
		mapping.recovered(listener.Address, listener.Port)
	}

	return mapping
}
