package securedaccess

import (
	"log"
	"strconv"

	corev1 "k8s.io/api/core/v1"

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

func (o *LoadbalancerAccessType) RealiseAndResolve(access *skupperv1alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv1alpha1.Endpoint, error) {
	log.Printf("Resolving endpoints for SecuredAccess %s of accessType 'loadbalancer'", access.Key())
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
	return endpoints, nil
}
