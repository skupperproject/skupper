package securedaccess

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type LocalAccessType struct {
	manager *SecuredAccessManager
}

func newLocalAccess(m *SecuredAccessManager) AccessType {
	return &LocalAccessType{
		manager: m,
	}
}

func (o *LocalAccessType) RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv2alpha1.Endpoint, error) {
	var endpoints []skupperv2alpha1.Endpoint
	for _, port := range access.Spec.Ports {
		endpoints = append(endpoints, skupperv2alpha1.Endpoint{
			Name: port.Name,
			Host: access.Name + "." + access.Namespace,
			Port: strconv.Itoa(port.Port),
		})
	}
	return endpoints, nil
}
