package claims

import (
	"context"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
)

type GrantManager interface {
	Start()
	GrantChanged(key string, grant *skupperv1alpha1.Grant) error
	SecuredAccessChanged(key string, se *skupperv1alpha1.SecuredAccess)
}

func NewGrantManager(clients kube.Clients, config *GrantConfig, generator GrantResponse) GrantManager {
	if !config.Enabled {
		return &GrantsDisabled{
			clients: clients,
		}
	}
	if config.SecuredAccessKey != "" {
		server := &UrlFromSecuredAccess{
			key: config.SecuredAccessKey,
		}
		server.configure(clients, config, generator)
		return server
	}
	server := &UrlFromEnv{}
	server.configure(clients, config, generator)
	return server
}

type UrlFromSecuredAccess struct {
	GrantServer
	key         string
	ready       bool
}

type UrlFromEnv struct {
	GrantServer
}

func (s *UrlFromEnv) SecuredAccessChanged(key string, se *skupperv1alpha1.SecuredAccess) {}

func (s *UrlFromEnv) Start() {
	s.start()
}

func (s *UrlFromSecuredAccess) SecuredAccessChanged(key string, se *skupperv1alpha1.SecuredAccess) {
	if se != nil && s.key == key && len(se.Status.Urls) > 0 && s.grants.url == "" {
		s.grants.url = se.Status.Urls[0].Url
		if s.ready {
			s.start()
		}
	}
}

func (s *UrlFromSecuredAccess) Start() {
	s.ready = true
	if s.grants.url != "" {
		s.start()
	}
}

const notEnabled string = "Grants are not enabled"

type GrantsDisabled struct {
	clients kube.Clients
}

func (s *GrantsDisabled) GrantChanged(key string, grant *skupperv1alpha1.Grant) error {
	if grant == nil || grant.Status.Status == notEnabled {
		return nil
	}
	grant.Status.Status = notEnabled
	if _, err := s.clients.GetSkupperClient().SkupperV1alpha1().Grants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{}); err != nil {
		log.Printf("%s. Error updating status for %s: %s", notEnabled, key, err)
	}
	return nil
}

func (s *GrantsDisabled) SecuredAccessChanged(key string, se *skupperv1alpha1.SecuredAccess) {}
func (s *GrantsDisabled) Start() {}
