package securedaccess

import (
	"fmt"
	"log"
	"reflect"

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
	log.Printf("Resolving URLs for SecuredAccess %s of accessType 'loadbalancer'", access.Key())
	svc, ok := o.manager.services[access.Key()]
	if !ok {
		log.Printf("Cannot resolve URLs; no service %s found", access.Key())
		return false
	}
	var urls []skupperv1alpha1.SecuredAccessUrl
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
			urls = append(urls, skupperv1alpha1.SecuredAccessUrl{
				Name: p.Name,
				Url:  fmt.Sprintf("%s:%d", host, p.Port),
			})
		}
	}
	log.Printf("Resolving URLs for SecuredAccess %s of accessType 'loadbalancer' -> %v", access.Key(), urls)
	if urls == nil || reflect.DeepEqual(urls, access.Status.Urls) {
		log.Printf("URLs for SecuredAccess %s of accessType 'loadbalancer' have not changed", access.Key())
		return false
	}
	access.Status.Urls = urls
	log.Printf("Resolved URLs for SecuredAccess %s of accessType 'loadbalancer' -> %v", access.Key(), urls)
	return true
}
