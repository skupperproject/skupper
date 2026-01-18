package controller

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type SystemAdaptor struct {
	agentPool *qdr.AgentPool
	namespace string
}

func NewSystemAdaptor(namespace string, agentPool *qdr.AgentPool) *SystemAdaptor {

	systemAdaptor := &SystemAdaptor{
		namespace: namespace,
		agentPool: agentPool,
	}
	return systemAdaptor
}

func (s *SystemAdaptor) syncWithRouter(desired *qdr.RouterConfig) error {
	if desired == nil {
		return nil
	}

	if err := s.syncSslProfileCredentialsToDisk(desired.SslProfiles); err != nil {
		return err
	}
	if err := qdr.SyncSslProfilesToRouter(s.agentPool, desired.SslProfiles); err != nil {
		return err
	}
	if err := qdr.SyncBridgeConfig(s.agentPool, &desired.Bridges); err != nil {
		log.Printf("sync failed: %s", err)
		return err
	}
	if err := qdr.SyncRouterConfig(s.agentPool, desired); err != nil {
		log.Printf("sync failed: %s", err)
		return err
	}
	return nil
}

// it should check that the ssl profiles have their respective credentials on disk
// TODO: implement certificate rotation like in kube environments
func (s *SystemAdaptor) syncSslProfileCredentialsToDisk(profiles map[string]qdr.SslProfile) error {

	namespacesPath := api.GetDefaultOutputNamespacesPath()
	for certificateName, _ := range profiles {
		tlsCertPath := path.Join(namespacesPath, s.namespace, string(api.CertificatesPath), certificateName)
		tlsCert := &runtime.TlsCert{
			CaPath:   path.Join(tlsCertPath, "ca.crt"),
			CertPath: path.Join(tlsCertPath, "tls.crt"),
			KeyPath:  path.Join(tlsCertPath, "tls.key"),
		}

		_, err := os.Stat(tlsCert.CaPath)
		if err != nil {
			return fmt.Errorf("%s not available for certificate %s: %s\n", tlsCert.CaPath, certificateName, err)
		}
		_, err = os.Stat(tlsCert.CertPath)
		if err != nil {
			return fmt.Errorf("%s not available for certificate %s: %s\n", tlsCert.CertPath, certificateName, err)
		}
		_, err = os.Stat(tlsCert.KeyPath)
		if err != nil {
			return fmt.Errorf("%s not available for certificate %s: %s\n", tlsCert.KeyPath, certificateName, err)
		}
	}

	return nil

}
