package podman

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/domain"
)

type TokenCertHandler struct{}

func (t *TokenCertHandler) Create(filename, subject string, info *domain.TokenCertInfo, site domain.Site, credHandler types.CredentialHandler) error {
	caSecret, err := credHandler.GetSecret(types.SiteCaSecret)
	if err != nil {
		return fmt.Errorf("error retrieving CA secret - %w", err)
	}

	// Generating the token certificate
	secret := certs.GenerateSecret(subject, subject, info.GetHosts(), caSecret)
	certs.AnnotateConnectionToken(&secret, "inter-router", info.InterRouterHost, info.InterRouterPort)
	certs.AnnotateConnectionToken(&secret, "edge", info.EdgeHost, info.EdgePort)
	secret.Annotations[types.SiteVersion] = site.GetVersion()

	// Adding qualifier
	if secret.ObjectMeta.Labels == nil {
		secret.ObjectMeta.Labels = map[string]string{}
	}
	secret.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
	secret.ObjectMeta.Annotations[types.TokenGeneratedBy] = site.GetId()

	// Writing as a file
	return certs.GenerateSecretFile(filename, &secret, false)
}
