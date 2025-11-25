package qdr

import (
	"log"
	"strconv"

	"github.com/skupperproject/skupper/internal/ports"
)

type PortMapping struct {
	mappings map[string]int
	pool     *ports.FreePorts
}

func (p *PortMapping) GetPortForKey(key string) (int, error) {
	if existing, ok := p.mappings[key]; ok {
		return existing, nil
	}
	allocated, err := p.pool.NextFreePort()
	if err != nil {
		return 0, err
	}
	p.mappings[key] = allocated
	log.Printf("Allocated port %d for key %s", allocated, key)
	return allocated, err
}

func (p *PortMapping) ReleasePortForKey(key string) {
	if existing, ok := p.mappings[key]; ok {
		p.pool.Release(existing)
		delete(p.mappings, key)
	}
}

func (p *PortMapping) recovered(key string, portstr string) {
	port, err := strconv.Atoi(portstr)
	if err != nil {
		log.Printf("Failed to convert port %q to int: %s", portstr, err)
		return
	}
	p.pool.InUse(port)
	p.mappings[key] = port
	if existing, ok := p.mappings[key]; ok {
		p.pool.Release(existing)
		delete(p.mappings, key)
	}
}

func RecoverPortMapping(config *RouterConfig) *PortMapping {
	mapping := &PortMapping{
		mappings: map[string]int{},
		pool:     ports.NewFreePorts(),
	}
	if config != nil {
		for _, listener := range config.Listeners {
			mapping.pool.InUse(int(listener.Port))
		}

		for key, listener := range config.Bridges.TcpListeners {
			mapping.recovered(key, listener.Port)
		}
	}
	return mapping
}
