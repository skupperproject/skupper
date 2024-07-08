package securedaccess

import (
	"log"

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

func (o *UnsupportedAccessType) Realise(access *skupperv1alpha1.SecuredAccess) bool {
	log.Printf("Unsupported access type %q in SecuredAccess %s/%s", access.Spec.AccessType, access.Namespace, access.Name)
	return access.Status.SetStatusMessage("unsupported access type")
}

func (o *UnsupportedAccessType) Resolve(access *skupperv1alpha1.SecuredAccess) bool {
	return false
}
