package securedaccess

import (
	"errors"
	"log"

	corev1 "k8s.io/api/core/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type UnsupportedAccessType struct {
	manager *SecuredAccessManager
}

func newUnsupportedAccess(m *SecuredAccessManager) AccessType {
	return &UnsupportedAccessType{
		manager: m,
	}
}

func (o *UnsupportedAccessType) RealiseAndResolve(access *skupperv1alpha1.SecuredAccess, service *corev1.Service) ([]skupperv1alpha1.Endpoint, error) {
	log.Printf("Unsupported access type %q in SecuredAccess %s/%s", access.Spec.AccessType, access.Namespace, access.Name)
	return nil, errors.New("unsupported access type")
}
