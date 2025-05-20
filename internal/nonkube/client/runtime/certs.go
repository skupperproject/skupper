package runtime

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path"

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
	retriever := &TlsConfigRetriever{
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

type TlsConfigRetriever struct {
	Cert   string
	Key    string
	Ca     string
	Verify bool
}

func (paths *TlsConfigRetriever) GetTlsConfig() (*tls.Config, error) {
	tlsConfig, err := getTlsConfig(paths.Verify, paths.Cert, paths.Key, paths.Ca)
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}

func getTlsConfig(verify bool, cert, key, ca string) (*tls.Config, error) {
	var config tls.Config
	config.InsecureSkipVerify = true
	if verify {
		certPool := x509.NewCertPool()
		file, err := os.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(file)
		config.RootCAs = certPool
		config.InsecureSkipVerify = false
	}

	_, errCert := os.Stat(cert)
	_, errKey := os.Stat(key)
	if errCert == nil || errKey == nil {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("could not load x509 key pair - %w", err)
		}
		config.Certificates = []tls.Certificate{tlsCert}
	}
	config.MinVersion = tls.VersionTLS10

	return &config, nil
}
