package securedaccess

import (
	corev1 "k8s.io/api/core/v1"

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

func (o *LocalAccessType) RealiseAndResolve(access *skupperv1alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv1alpha1.Endpoint, error) {
	return []skupperv1alpha1.Endpoint{} /*return empty slice rather than nil so that it will be treated as resolved*/, nil
}
