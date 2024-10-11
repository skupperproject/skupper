package securedaccess

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type NodeportAccessType struct {
	manager     *SecuredAccessManager
	clusterHost string
}

func newNodeportAccess(m *SecuredAccessManager, clusterHost string) AccessType {
	return &NodeportAccessType{
		manager:     m,
		clusterHost: clusterHost,
	}
}

func (o *NodeportAccessType) RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv2alpha1.Endpoint, error) {
	var endpoints []skupperv2alpha1.Endpoint
	for _, p := range svc.Spec.Ports {
		if p.NodePort != 0 {
			endpoints = append(endpoints, skupperv2alpha1.Endpoint{
				Name: p.Name,
				Host: o.clusterHost,
				Port: strconv.Itoa(int(p.NodePort)),
			})
		}
	}
	return endpoints, nil
}
