package data

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Service struct {
	Address  string          `json:"address"`
	Protocol string          `json:"protocol"`
	Targets  []ServiceTarget `json:"targets"`
}

type ServiceTarget struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	SiteId string `json:"site_id"`
}

func (s *Service) AddTarget(name string, host string, siteId string, mapping NameMapping) {
	target := ServiceTarget{
		Name:   mapping.Lookup(host),
		Target: getTargetNameFromConnectorName(name),
		SiteId: siteId,
	}
	s.Targets = append(s.Targets, target)
}

func (s *Service) AddressUnqualified() string {
	return unqualifiedAddress(s.Address)
}

func unqualifiedAddress(address string) string {
	return strings.Split(address, ":")[0]
}

type IngressBinding struct {
	ListenerPorts   map[int]int       `json:"listener_ports"`
	ServicePorts    map[int]int       `json:"service_ports"`
	ServiceSelector map[string]string `json:"service_selector"`
}

type EgressBinding struct {
	Ports map[int]int `json:"ports"`
	Host  string      `json:"host"`
}

type ServiceDetail struct {
	SiteId         string                 `json:"site_id"`
	Definition     types.ServiceInterface `json:"definition"`
	IngressBinding IngressBinding         `json:"ingress_binding"`
	EgressBindings []EgressBinding        `json:"egress_bindings"`
	Observations   []string               `json:"observations,omitempty"`
}

type ServiceCheck struct {
	Details      []ServiceDetail `json:"site_details"`
	Observations []string        `json:"observations,omitempty"`
}

func (sd *ServiceDetail) AddObservation(message string) {
	sd.Observations = append(sd.Observations, message)
}

func (sc *ServiceCheck) AddObservation(message string) {
	sc.Observations = append(sc.Observations, message)
}

func (sc *ServiceCheck) HasDetailObservations() bool {
	for _, d := range sc.Details {
		if len(d.Observations) > 0 {
			return true
		}
	}
	return false
}

func CheckService(details *ServiceCheck) {
	//consistency checking
	//- do definitions from different sites match?
	for i := 0; i+1 < len(details.Details); i++ {
		aSiteId := details.Details[i].SiteId
		bSiteId := details.Details[i+1].SiteId
		a := &details.Details[i].Definition
		b := &details.Details[i+1].Definition
		if a.Address != "" && b.Address != "" {
			if a.Address != b.Address {
				details.AddObservation(fmt.Sprintf("Mismatched address between sites %s and %s (%s != %s)", aSiteId, bSiteId, a.Address, b.Address))
			}
			if a.Protocol != b.Protocol {
				details.AddObservation(fmt.Sprintf("Mismatched protocol between sites %s and %s (%s != %s)", aSiteId, bSiteId, a.Protocol, b.Protocol))
			}
			if !reflect.DeepEqual(a.Ports, b.Ports) {
				details.AddObservation(fmt.Sprintf("Different ports used in sites %s (%v) and %s (%v)", aSiteId, a.Ports, bSiteId, b.Ports))
			}
		}
	}
	//- do all sites have a correctly defined ingress binding?
	//- do all sites with a target defined have at least one egress binding?
	egressCount := 0
	for _, site := range details.Details {
		for _, port := range site.Definition.Ports {
			serviceTarget, ok := site.IngressBinding.ServicePorts[port]
			if !ok {
				details.AddObservation(fmt.Sprintf("Ingress binding for site %s does not have a service port for %d", site.SiteId, port))
			}
			listenerTarget, ok := site.IngressBinding.ListenerPorts[port]
			if !ok {
				details.AddObservation(fmt.Sprintf("Ingress binding for site %s does not have a listener for %d", site.SiteId, port))
			}
			if listenerTarget != serviceTarget {
				details.AddObservation(fmt.Sprintf("In ingress binding for site %s, target port on service (%d) does not match listener port (%d)", site.SiteId, serviceTarget, listenerTarget))
			}
		}
		for _, egress := range site.EgressBindings {
			if egress.Host != "" && len(egress.Ports) != 0 {
				egressCount++
			}
		}
	}
	//- is there at least one egress binding?
	if egressCount == 0 {
		details.AddObservation("There are no egress bindings")
	}
}

func getTargetNameFromConnectorName(name string) string {
	return strings.Split(strings.Split(name, "@")[0], ".")[0]
}

func stripPort(address string) (string, int, error) {
	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("Address does not include port: %q", address)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("Invalid port in %q: %q", address, parts[1])
	}
	return parts[0], port, nil
}

func (detail *ServiceDetail) extractListenerPort(name string, address string, port string) {
	if _, logicalPort, err := stripPort(address); err == nil {
		listenerPort, err := strconv.Atoi(port)
		if err != nil {
			detail.AddObservation(fmt.Sprintf("Bad port for listener %s: %s %s", name, port, err))
		}
		detail.IngressBinding.ListenerPorts[logicalPort] = listenerPort
	} else {
		detail.AddObservation(fmt.Sprintf("Invalid address %q for listener %s: %s", address, name, err))
	}
}

func (detail *ServiceDetail) ExtractTcpListenerPorts(listeners []qdr.TcpEndpoint) {
	detail.IngressBinding.ListenerPorts = map[int]int{}
	for _, listener := range listeners {
		detail.extractListenerPort(listener.Name, listener.Address, listener.Port)
	}
}

func (detail *ServiceDetail) ExtractHttpListenerPorts(listeners []qdr.HttpEndpoint) {
	detail.IngressBinding.ListenerPorts = map[int]int{}
	for _, listener := range listeners {
		detail.extractListenerPort(listener.Name, listener.Address, listener.Port)
	}
}

type EgressPortMap map[string]map[int]int

func (hosts EgressPortMap) handle(name string, address string, port string) error {
	unqualified, logicalPort, err := stripPort(address)
	if err != nil {
		return fmt.Errorf("Invalid address %q for connector %s: %s", address, name, err)
	}
	targetPort, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("Bad port for connector %s: %s %s", name, port, err)
	}
	if hosts[unqualified] == nil {
		hosts[unqualified] = map[int]int{}
	}
	hosts[unqualified][logicalPort] = targetPort
	return nil
}

func (hosts EgressPortMap) asEgressBindings() []EgressBinding {
	bindings := []EgressBinding{}
	for host, ports := range hosts {
		bindings = append(bindings, EgressBinding{
			Ports: ports,
			Host:  host,
		})
	}
	return bindings
}

func (detail *ServiceDetail) ExtractTcpConnectorPorts(connectors []qdr.TcpEndpoint) {
	hosts := EgressPortMap{}
	for _, connector := range connectors {
		if err := hosts.handle(connector.Name, connector.Address, connector.Port); err != nil {
			detail.AddObservation(err.Error())
		}
	}
	detail.EgressBindings = hosts.asEgressBindings()
}

func (detail *ServiceDetail) ExtractHttpConnectorPorts(connectors []qdr.HttpEndpoint) {
	hosts := EgressPortMap{}
	for _, connector := range connectors {
		if err := hosts.handle(connector.Name, connector.Address, connector.Port); err != nil {
			detail.AddObservation(err.Error())
		}
	}
	detail.EgressBindings = hosts.asEgressBindings()
}
