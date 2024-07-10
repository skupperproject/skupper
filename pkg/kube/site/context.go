package site

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
)

type SiteContext struct {
	clients         internalclient.Clients
	namespace       string
	siteConfig      *types.SiteConfig
	routerConfig    *qdr.RouterConfig
	ownerReferences []metav1.OwnerReference
	resolver        resolver.Resolver
}

func GetSiteContext(clients internalclient.Clients, namespace string, ctx context.Context) (*SiteContext, error) {
	impl := &SiteContext{
		clients:   clients,
		namespace: namespace,
	}
	err := impl.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return impl, nil
}

func (s *SiteContext) LoadConfig(ctx context.Context) error {
	sconfig, err := s.getConfigMap(ctx, types.SiteConfigMapName)
	if err != nil {
		return err
	}
	siteConfig, _ := site.ReadSiteConfig(sconfig, s.defaultIngress())

	rconfig, err := s.getConfigMap(ctx, types.TransportConfigMapName)
	if err != nil {
		return err
	}
	routerConfig, err := qdr.GetRouterConfigFromConfigMap(rconfig)
	if err != nil {
		return err
	}
	resolver, err := resolver.NewResolver(s.clients, s.namespace, &siteConfig.Spec)
	if err != nil {
		return err
	}

	s.siteConfig = siteConfig
	s.routerConfig = routerConfig
	s.ownerReferences = rconfig.ObjectMeta.OwnerReferences
	s.resolver = resolver
	return nil

}

func (s *SiteContext) defaultIngress() string {
	return defaultIngress(s.clients)
}

func defaultIngress(clients internalclient.Clients) string {
	if clients.GetRouteClient() == nil {
		return types.IngressLoadBalancerString
	}
	return types.IngressRouteString
}

func (s *SiteContext) getConfigMap(ctx context.Context, name string) (*corev1.ConfigMap, error) {
	client := s.clients.GetKubeClient()
	config, err := client.CoreV1().ConfigMaps(s.namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, fmt.Errorf("Skupper is not installed in %s", s.namespace)
	} else if err != nil {
		return nil, err
	}
	return config, nil
}

func (s *SiteContext) GetOwnerReferences() []metav1.OwnerReference {
	return s.ownerReferences
}

func (s *SiteContext) IsEdge() bool {
	return s.routerConfig.IsEdge()
}

func (s *SiteContext) GetSiteVersion() string {
	return s.routerConfig.GetSiteMetadata().Version
}

func (s *SiteContext) GetSiteId() string {
	return s.siteConfig.Reference.UID
}

func (s *SiteContext) IsLocalAccessOnly() bool {
	return s.resolver.IsLocalAccessOnly()
}

func (s *SiteContext) GetAllHosts() ([]string, error) {
	return s.resolver.GetAllHosts()
}

func (s *SiteContext) GetHostPortForInterRouter() (resolver.HostPort, error) {
	return s.resolver.GetHostPortForInterRouter()
}

func (s *SiteContext) GetHostPortForEdge() (resolver.HostPort, error) {
	return s.resolver.GetHostPortForEdge()
}

func (s *SiteContext) GetHostPortForClaims() (resolver.HostPort, error) {
	return s.resolver.GetHostPortForClaims()
}
