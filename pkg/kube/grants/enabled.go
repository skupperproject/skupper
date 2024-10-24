package grants

import (
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
)

func enabled(controller *kube.Controller, currentNamespace string, watchNamespace string, config *GrantConfig, generator GrantResponse) *GrantsEnabled {
	gc := &GrantsEnabled{
		grants: newGrants(controller, generator, config.scheme(), config.BaseUrl),
	}
	gc.server = newServer(config.addr(), config.tlsEnabled(), gc.grants)

	gc.grantWatcher = controller.WatchAccessGrants(watchNamespace, gc.grants.checkGrant)
	gc.secretWatcher = controller.WatchSecrets(kube.ByName(config.TlsCredentialsSecret), watchNamespace, gc.tlsCredentialsUpdated)

	if config.AutoConfigure {
		ac, err := newAutoConfigure(gc.securedAccessChanged, controller, currentNamespace, config)
		if err != nil {
			log.Printf("Auto configuration of grant server failed: %s", err)
		}
		gc.autoConfigure = ac
	}
	return gc
}

type GrantsEnabled struct {
	grants        *Grants
	server        *Server
	grantWatcher  *kube.AccessGrantWatcher
	secretWatcher *kube.SecretWatcher
	autoConfigure *AutoConfigure
	started       bool
}

func (c *GrantsEnabled) Start() {
	c.recoverGrants()
	c.recoverSecrets()
	if c.autoConfigure == nil {
		c.server.start()
	}
}

func (c *GrantsEnabled) recoverGrants() {
	for _, grant := range c.grantWatcher.List() {
		c.grants.checkGrant(fmt.Sprintf("%s/%s", grant.Namespace, grant.Name), grant)
	}
}

func (c *GrantsEnabled) recoverSecrets() {
	for _, secret := range c.secretWatcher.List() {
		c.tlsCredentialsUpdated(fmt.Sprintf("%s/%s", secret.Namespace, secret.Name), secret)
	}
}

func (s *GrantsEnabled) securedAccessChanged(key string, se *skupperv2alpha1.SecuredAccess) error {
	if se != nil && len(se.Status.Endpoints) > 0 {
		if s.grants.setUrl(se.Status.Endpoints[0].Url()) {
			s.grants.recheckUrl()
		}
		if !s.started {
			s.started = true
			log.Print("Starting grant server")
			s.server.start()
		}
	}
	return nil
}

func (s *GrantsEnabled) tlsCredentialsUpdated(key string, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	if err := s.server.setCertificateFromSecret(secret); err != nil {
		log.Printf("Could not set certificate from grant server from %s: %s", key, err)
		return nil
	}
	if s.grants.setCA(string(secret.Data["ca.crt"])) {
		s.grants.recheckCa()
	}
	log.Print("Grant server tls credentials updated")
	return nil
}
