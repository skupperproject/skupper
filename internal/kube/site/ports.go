package site

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func (p *ExposedPortSet) empty() bool {
	return len(p.Ports) == 0
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
		if existing.empty() {
			delete(p, host)
		}
		return existing
	}
	//no change was required
	return nil
}

func toServicePorts(desired map[string]Port) map[string]corev1.ServicePort {
	results := map[string]corev1.ServicePort{}
	for name, details := range desired {
		results[name] = corev1.ServicePort{
			Name:       name,
			Port:       int32(details.Port),
			TargetPort: intstr.IntOrString{IntVal: int32(details.TargetPort)},
			Protocol:   details.Protocol,
		}
	}
	return results
}

func updatePorts(spec *corev1.ServiceSpec, desired map[string]Port) bool {
	expected := toServicePorts(desired)
	changed := false
	var ports []corev1.ServicePort
	for _, actual := range spec.Ports {
		if port, ok := expected[actual.Name]; ok {
			ports = append(ports, port)
			delete(expected, actual.Name)
			if actual != port {
				changed = true
			}
		} else {
			changed = true
		}
	}
	for _, port := range expected {
		ports = append(ports, port)
		changed = true
	}
	if changed {
		spec.Ports = ports
	}
	return changed
}
