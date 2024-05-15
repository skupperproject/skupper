package securedaccess

import (
	"log"
	"reflect"
	"strconv"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type LoadbalancerAccessType struct {
	manager *SecuredAccessManager
}

func newLoadbalancerAccess(m *SecuredAccessManager) AccessType {
	return &LoadbalancerAccessType{
		manager: m,
	}
}

func (o *LoadbalancerAccessType) Realise(access *skupperv1alpha1.SecuredAccess) bool {
	if access.Status.Status == "OK" {
		return false
	}
	access.Status.Status = "OK"
	return true
}

func (o *LoadbalancerAccessType) Resolve(access *skupperv1alpha1.SecuredAccess) bool {
	log.Printf("Resolving endpoints for SecuredAccess %s of accessType 'loadbalancer'", access.Key())
	svc, ok := o.manager.services[access.Key()]
	if !ok {
		log.Printf("Cannot resolve endpoints; no service %s found", access.Key())
		return false
	}
	var endpoints []skupperv1alpha1.Endpoint
	for _, i := range svc.Status.LoadBalancer.Ingress {
		var host string
		if i.IP != "" {
			host = i.IP
		} else if i.Hostname != "" {
			host = i.Hostname
		} else {
			continue
		}
		for _, p := range svc.Spec.Ports {
			endpoints = append(endpoints, skupperv1alpha1.Endpoint{
				Name: p.Name,
				Host: host,
				Port: strconv.Itoa(int(p.Port)),
			})
		}
	}
	log.Printf("Resolving endpoints for SecuredAccess %s of accessType 'loadbalancer' -> %v", access.Key(), endpoints)
	if endpoints == nil || reflect.DeepEqual(endpoints, access.Status.Endpoints) {
		log.Printf("Endpoints for SecuredAccess %s of accessType 'loadbalancer' have not changed", access.Key())
		return false
	}
	access.Status.Endpoints = endpoints
	log.Printf("Resolved endpoints for SecuredAccess %s of accessType 'loadbalancer' -> %v", access.Key(), endpoints)
	return true
}
