package qdr

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/internal/ports"
)

type PortMapping struct {
	mappings map[string]int
	Pool     *ports.FreePorts
	logger   *slog.Logger
}

func (p *PortMapping) GetAllocatedPorts() map[int]string {
	ports := map[int]string{}
	for key, port := range p.mappings {
		ports[port] = key
	}
	return ports
}

func (p *PortMapping) GetPortForKey(key string) (int, error) {
	if existing, ok := p.mappings[key]; ok {
		return existing, nil
	}
	allocated, err := p.Pool.NextFreePort()
	if err != nil {
		return 0, err
	}
	p.mappings[key] = allocated
	p.logger.Info("Allocated port for key", slog.Int("port", allocated), slog.String("key", key))
	return allocated, err
}

func (p *PortMapping) ReleasePortForKey(key string) {
	if existing, ok := p.mappings[key]; ok {
		p.Pool.Release(existing)
		delete(p.mappings, key)
	}
}

func (p *PortMapping) recovered(key string, portstr string) {
	port, err := strconv.Atoi(portstr)
	if err != nil {
		p.logger.Error("Failed to convert port to int", slog.String("port", portstr), slog.Any("error", err))
		return
	}
	p.Pool.InUse(port)
	p.mappings[key] = port
}

func portMappingKey(listener TcpEndpoint) string {
	if strings.HasPrefix(listener.Name, TcpListenerNamePrefix) {
		name := strings.TrimPrefix(listener.Name, TcpListenerNamePrefix)
		if strings.Contains(name, "@") && listener.Address != "" {
			return listener.Address
		}
		return name
	}
	if strings.HasPrefix(listener.Name, "multiAddress/") {
		return "multiaddress-" + strings.TrimPrefix(listener.Name, "multiAddress/")
	}
	return listener.Name
}

func RecoverPortMapping(config *RouterConfig) *PortMapping {
	mapping := &PortMapping{
		mappings: map[string]int{},
		Pool:     ports.NewFreePorts(),
		logger:   slog.New(slog.Default().Handler()).With(slog.String("component", "qdr.portMapping")),
	}
	if config != nil {
		for _, listener := range config.Listeners {
			mapping.Pool.InUse(int(listener.Port))
		}

		for _, listener := range config.Bridges.TcpListeners {
			mapping.recovered(portMappingKey(listener), listener.Port)
		}
	}
	return mapping
}
