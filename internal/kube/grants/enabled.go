package grants

import (
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type NamespaceFilter func(string) bool

func enabled(controller *internalclient.Controller, currentNamespace string, watchNamespace string, config *GrantConfig, generator GrantResponse, filter NamespaceFilter) *GrantsEnabled {
	gc := &GrantsEnabled{
		grants: newGrants(controller, generator, config.scheme(), config.BaseUrl),
	}
	gc.server = newServer(config.addr(), config.tlsEnabled(), gc.grants)

	gc.grantWatcher = controller.WatchAccessGrants(watchNamespace, internalclient.FilterByNamespace(filter, gc.grants.checkGrant))
	gc.secretWatcher = controller.WatchSecrets(internalclient.ByName(config.TlsCredentialsSecret), watchNamespace, internalclient.FilterByNamespace(filter, gc.tlsCredentialsUpdated))

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
	grantWatcher  *internalclient.AccessGrantWatcher
	secretWatcher *internalclient.SecretWatcher
	autoConfigure *AutoConfigure
	started       bool
	filter        NamespaceFilter
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
		if c.filter != nil && !c.filter(grant.Namespace) {
			continue
		}
		c.grants.checkGrant(fmt.Sprintf("%s/%s", grant.Namespace, grant.Name), grant)
	}
}

func (c *GrantsEnabled) recoverSecrets() {
	for _, secret := range c.secretWatcher.List() {
		if c.filter != nil && !c.filter(secret.Namespace) {
			continue
		}
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
