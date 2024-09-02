package securedaccess

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
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

func (o *NodeportAccessType) RealiseAndResolve(access *skupperv1alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv1alpha1.Endpoint, error) {
	var endpoints []skupperv1alpha1.Endpoint
	for _, p := range svc.Spec.Ports {
		endpoints = append(endpoints, skupperv1alpha1.Endpoint{
			Name: p.Name,
			Host: o.clusterHost,
			Port: strconv.Itoa(int(p.NodePort)),
		})
	}
	return endpoints, nil
}
