package controller

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	"github.com/skupperproject/skupper/internal/qdr"
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
	if err := qdr.SyncSslProfilesToRouter(s.agentPool, s.addSslPathToProfileCredentials(desired.SslProfiles)); err != nil {
		return err
	}
	if err := qdr.SyncBridgeConfig(s.agentPool, &desired.Bridges); err != nil {
		log.Printf("sync failed: %s", err)
		return err
	}

	//Do not double-check that certificates exist; it has been done by previous syncSslProfileCredentialsToDisk
	// Also, the paths included in the ssl profiles are relative to the router instead of the runtime directory
	if err := qdr.SyncRouterConfig(s.agentPool, desired, false); err != nil {
		log.Printf("sync failed: %s", err)
		return err
	}
	return nil
}

// it should check that the ssl profiles have their respective credentials on disk
// TODO: implement certificate rotation like in kube environments
func (s *SystemAdaptor) syncSslProfileCredentialsToDisk(profiles map[string]qdr.SslProfile) error {

	for certificateName, _ := range profiles {
		tlsCert := runtime.GetRuntimeTlsCert(s.namespace, certificateName)

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

func (s *SystemAdaptor) addSslPathToProfileCredentials(profiles map[string]qdr.SslProfile) map[string]qdr.SslProfile {

	completedProfiles := make(map[string]qdr.SslProfile)
	sslProfilePath := "/etc/skupper-router/runtime/certs"

	for certificateName, profile := range profiles {

		profile.CaCertFile = path.Join(sslProfilePath, certificateName, "ca.crt")
		profile.CertFile = path.Join(sslProfilePath, certificateName, "tls.crt")
		profile.PrivateKeyFile = path.Join(sslProfilePath, certificateName, "tls.key")

		completedProfiles[certificateName] = profile
	}

	return completedProfiles

}
