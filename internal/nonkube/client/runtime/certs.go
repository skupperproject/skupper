package runtime

import (
	"crypto/tls"
	"path"

	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

var (
	NamespacesPath string
)

type TlsCert struct {
	CaPath   string
	CertPath string
	KeyPath  string
	Verify   bool
	config   *tls.Config
}

func (t *TlsCert) GetTlsConfig() (*tls.Config, error) {
	var err error
	if t.config == nil {
		err = t.LoadTlsConfig()
	}
	return t.config, err
}

func (t *TlsCert) LoadTlsConfig() error {
	retriever := &certs.TlsConfigRetriever{
		Cert:   t.CertPath,
		Key:    t.KeyPath,
		Ca:     t.CaPath,
		Verify: t.Verify,
	}
	var err error
	t.config, err = retriever.GetTlsConfig()
	if err != nil {
		return err
	}
	return nil
}

func GetRuntimeTlsCert(namespace, certificateName string) *TlsCert {
	namespacesPath := NamespacesPath
	if namespacesPath == "" {
		namespacesPath = api.GetDefaultOutputNamespacesPath()
	}
	tlsCertPath := path.Join(namespacesPath, namespace, string(api.CertificatesPath), certificateName)
	tlsCert := &TlsCert{
		CaPath:   path.Join(tlsCertPath, "ca.crt"),
		CertPath: path.Join(tlsCertPath, "tls.crt"),
		KeyPath:  path.Join(tlsCertPath, "tls.key"),
		Verify:   true,
	}
	return tlsCert
}
