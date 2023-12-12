package domain

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"k8s.io/api/core/v1"
)

type TokenCertInfo struct {
	InterRouterHost string
	InterRouterPort string
	EdgeHost        string
	EdgePort        string
}

func (t *TokenCertInfo) GetHosts() string {
	return fmt.Sprintf("%s,%s", t.InterRouterHost, t.EdgeHost)
}

type TokenCertHandler interface {
	Create(filename, subject string, info *TokenCertInfo, site Site, credHandler types.CredentialHandler) error
}

func VerifyToken(secret *v1.Secret) error {
	if secret.ObjectMeta.Labels == nil {
		secret.ObjectMeta.Labels = map[string]string{}
	}
	if _, ok := secret.ObjectMeta.Labels[types.SkupperTypeQualifier]; !ok {
		// deduce type from structure of secret
		if _, ok = secret.Data["tls.crt"]; ok {
			secret.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
		} else if secret.ObjectMeta.Annotations != nil && secret.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey] != "" {
			secret.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeClaimRequest
		}
	}
	switch secret.ObjectMeta.Labels[types.SkupperTypeQualifier] {
	case types.TypeToken:
		CertTokenDataFields := []string{"tls.key", "tls.crt", "ca.crt"}
		if secret.ObjectMeta.Annotations != nil && secret.ObjectMeta.Annotations[types.TokenTemplate] != "" {
			CertTokenDataFields = []string{"ca.crt"}
		}
		for _, name := range CertTokenDataFields {
			if _, ok := secret.Data[name]; !ok {
				return fmt.Errorf("Expected %s field in secret data", name)
			}
		}
	case types.TypeClaimRequest:
		if _, ok := secret.Data["password"]; !ok {
			return fmt.Errorf("Expected password field in secret data")
		}
		if secret.ObjectMeta.Annotations == nil || secret.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey] == "" {
			return fmt.Errorf("Expected %s annotation", types.ClaimUrlAnnotationKey)
		}
	default:
		return fmt.Errorf("Secret is not a valid skupper token")
	}
	return nil
}
