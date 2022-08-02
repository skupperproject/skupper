package site

import (
	corev1 "k8s.io/api/core/v1"
)

type Port struct {
	Name       string
	Port       int
	TargetPort int
	Protocol   corev1.Protocol
}

type ExposedPortSet struct {
	Host  string
	Ports map[string]Port
}

func (p *ExposedPortSet) add(port Port) bool {
	if existing, ok := p.Ports[port.Name]; !ok || existing != port {
		p.Ports[port.Name] = port
		return true
	}
	return false
}

func (p *ExposedPortSet) remove(portname string) bool {
	if _, ok := p.Ports[portname]; ok {
		delete(p.Ports, portname)
		return true
	}
	return false
}

type ExposedPorts map[string]*ExposedPortSet

func (p ExposedPorts) Expose(host string, port Port) *ExposedPortSet {
	if existing, ok := p[host]; ok {
		if existing.add(port) {
			return existing
		} else {
			//no change was needed
			return nil
		}
	} else {
		portset := &ExposedPortSet{
			Host: host,
			Ports: map[string]Port{
				port.Name: port,
			},
		}
		p[host] = portset
		return portset
	}
}

func (p ExposedPorts) Unexpose(host string, portname string) *ExposedPortSet {
	if existing, ok := p[host]; ok && existing.remove(portname) {
		return existing
	}
	//no change was required
	return nil
}
