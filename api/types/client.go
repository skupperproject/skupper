package types

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

type ConnectorCreateOptions struct {
	SkupperNamespace string
	Name             string
	Cost             int32
}

type ConnectorRemoveOptions struct {
	SkupperNamespace string
	Name             string
	ForceCurrent     bool
}

type ConnectorInspectResponse struct {
	SkupperNamespace string
	Connector        *Connector
	Connected        bool
}

type SiteConfig struct {
	Spec      SiteConfigSpec
	Reference SiteConfigReference
}

type RouterLogConfig struct {
	Module string
	Level  string
}

type SiteConfigSpec struct {
	SkupperName         string
	SkupperNamespace    string
	RouterMode          string
	EnableController    bool
	EnableServiceSync   bool
	EnableRouterConsole bool
	EnableConsole       bool
	AuthMode            string
	User                string
	Password            string
	Ingress             string
	Replicas            int32
	SiteControlled      bool
	RouterLogging       []RouterLogConfig
	RouterDebugMode     string
}

const (
	IngressRouteString        string = "route"
	IngressLoadBalancerString string = "loadbalancer"
	IngressNoneString         string = "none"
)

func (s *SiteConfigSpec) IsIngressRoute() bool {
	return s.Ingress == IngressRouteString
}
func (s *SiteConfigSpec) IsIngressLoadBalancer() bool {
	return s.Ingress == IngressLoadBalancerString
}
func (s *SiteConfigSpec) IsIngressNone() bool {
	return s.Ingress == IngressNoneString
}

type SiteConfigReference struct {
	UID        string
	Name       string
	APIVersion string
	Kind       string
}

type ServiceInterfaceCreateOptions struct {
	Protocol   string
	Address    string
	Port       int
	TargetPort int
	Headless   bool
}

type RouterInspectResponse struct {
	Status            RouterStatusSpec
	TransportVersion  string
	ControllerVersion string
	ExposedServices   int
	ConsoleUrl        string
}

type VanClientInterface interface {
	RouterCreate(ctx context.Context, options SiteConfig) error
	RouterInspect(ctx context.Context) (*RouterInspectResponse, error)
	RouterInspectNamespace(ctx context.Context, namespace string) (*RouterInspectResponse, error)
	RouterRemove(ctx context.Context) error
	RouterUpdateVersion(ctx context.Context, hup bool) (bool, error)
	RouterUpdateVersionInNamespace(ctx context.Context, hup bool, namespace string) (bool, error)
	ConnectorCreateFromFile(ctx context.Context, secretFile string, options ConnectorCreateOptions) (*corev1.Secret, error)
	ConnectorCreateSecretFromFile(ctx context.Context, secretFile string, options ConnectorCreateOptions) (*corev1.Secret, error)
	ConnectorCreate(ctx context.Context, secret *corev1.Secret, options ConnectorCreateOptions) error
	ConnectorInspect(ctx context.Context, name string) (*ConnectorInspectResponse, error)
	ConnectorList(ctx context.Context) ([]*Connector, error)
	ConnectorRemove(ctx context.Context, options ConnectorRemoveOptions) error
	ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error)
	ConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error
	ServiceInterfaceCreate(ctx context.Context, service *ServiceInterface) error
	ServiceInterfaceInspect(ctx context.Context, address string) (*ServiceInterface, error)
	ServiceInterfaceList(ctx context.Context) ([]*ServiceInterface, error)
	ServiceInterfaceRemove(ctx context.Context, address string) error
	ServiceInterfaceUpdate(ctx context.Context, service *ServiceInterface) error
	ServiceInterfaceBind(ctx context.Context, service *ServiceInterface, targetType string, targetName string, protocol string, targetPort int) error
	GetHeadlessServiceConfiguration(targetName string, protocol string, address string, port int) (*ServiceInterface, error)
	ServiceInterfaceUnbind(ctx context.Context, targetType string, targetName string, address string, deleteIfNoTargets bool) error
	SiteConfigCreate(ctx context.Context, spec SiteConfigSpec) (*SiteConfig, error)
	SiteConfigUpdate(ctx context.Context, spec SiteConfigSpec) ([]string, error)
	SiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*SiteConfig, error)
	SiteConfigRemove(ctx context.Context) error
	SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) error
	GetNamespace() string
	GetVersion(component string, name string) string
	GetIngressDefault() string
}
