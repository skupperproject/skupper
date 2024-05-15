package securedaccess

import (
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type LocalAccessType struct {
	manager *SecuredAccessManager
}

func newLocalAccess(m *SecuredAccessManager) AccessType {
	return &LocalAccessType{
		manager: m,
	}
}

func (o *LocalAccessType) Realise(access *skupperv1alpha1.SecuredAccess) bool {
	if access.Status.Status == "OK" {
		return false
	}
	access.Status.Status = "OK"
	return true
}

func (o *LocalAccessType) Resolve(access *skupperv1alpha1.SecuredAccess) bool {
	return false
}
