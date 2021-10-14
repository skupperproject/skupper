package data

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/skupperproject/skupper/api/types"
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
	ListenerPort      int               `json:"listener_port"`
	ServicePort       int               `json:"service_port"`
	ServiceTargetPort int               `json:"service_target_port"`
	ServiceSelector   map[string]string `json:"service_selector"`
}

type EgressBinding struct {
	Port int    `json:"port"`
	Host string `json:"host"`
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
		if site.IngressBinding.ListenerPort == 0 {
			details.AddObservation(fmt.Sprintf("No valid ingress binding for site %s, listener port not set", site.SiteId))
		}
		if site.IngressBinding.ServicePort == 0 {
			details.AddObservation(fmt.Sprintf("No valid ingress binding for site %s, service port not set", site.SiteId))
		}
		if site.IngressBinding.ServiceTargetPort != site.IngressBinding.ListenerPort {
			details.AddObservation(fmt.Sprintf("Invalid ingress binding for site %s, target port on service does not match listener port", site.SiteId))
		}
		for _, egress := range site.EgressBindings {
			if egress.Host != "" && egress.Port != 0 {
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
