package securedaccess

import (
	"errors"
	"log/slog"

	corev1 "k8s.io/api/core/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type UnsupportedAccessType struct {
	manager *SecuredAccessManager
	logger  *slog.Logger
}

func newUnsupportedAccess(m *SecuredAccessManager) AccessType {
	return &UnsupportedAccessType{
		manager: m,
		logger:  slog.New(slog.Default().Handler()).With(slog.String("component", "kube.securedaccess.unsupportedAccessType")),
	}
}

func (o *UnsupportedAccessType) RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, service *corev1.Service) ([]skupperv2alpha1.Endpoint, error) {
	o.logger.Info("Unsupported access type in SecuredAccess",
		slog.String("accessType", access.Spec.AccessType),
		slog.String("namespace", access.Namespace),
		slog.String("name", access.Name))
	return nil, errors.New("unsupported access type")
}
