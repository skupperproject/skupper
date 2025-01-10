package securedaccess

import (
	"errors"
	"log"

	corev1 "k8s.io/api/core/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type UnsupportedAccessType struct {
	manager *SecuredAccessManager
}

func newUnsupportedAccess(m *SecuredAccessManager) AccessType {
	return &UnsupportedAccessType{
		manager: m,
	}
}

func (o *UnsupportedAccessType) RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, service *corev1.Service) ([]skupperv2alpha1.Endpoint, error) {
	log.Printf("Unsupported access type %q in SecuredAccess %s/%s", access.Spec.AccessType, access.Namespace, access.Name)
	return nil, errors.New("unsupported access type")
}
