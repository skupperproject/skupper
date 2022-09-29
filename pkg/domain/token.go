package domain

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
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
