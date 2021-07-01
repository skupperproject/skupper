package types

import (
	"context"
	"fmt"

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

type Tuning struct {
	NodeSelector string
	Affinity     string
	AntiAffinity string
	Cpu          string
	Memory       string
}

type RouterOptions struct {
	Tuning
	Logging          []RouterLogConfig
	DebugMode        string
	MaxFrameSize     int
	MaxSessionFrames int
}

type ControllerOptions struct {
	Tuning
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
	ConsoleIngress      string
	Replicas            int32
	SiteControlled      bool
	Annotations         map[string]string
	Labels              map[string]string
	Router              RouterOptions
	Controller          ControllerOptions
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

func (s *SiteConfigSpec) IsConsoleIngressRoute() bool {
	return s.getConsoleIngress() == IngressRouteString
}
func (s *SiteConfigSpec) IsConsoleIngressLoadBalancer() bool {
	return s.getConsoleIngress() == IngressLoadBalancerString
}
func (s *SiteConfigSpec) IsConsoleIngressNone() bool {
	return s.getConsoleIngress() == IngressNoneString
}
func (s *SiteConfigSpec) getConsoleIngress() string {
	if s.ConsoleIngress == "" {
		return s.Ingress
	}
	return s.ConsoleIngress
}

func isValidIngress(ingress string) bool {
	return ingress == "" || ingress == IngressRouteString || ingress == IngressLoadBalancerString || ingress == IngressNoneString
}

func (s *SiteConfigSpec) CheckIngress() error {
	if !isValidIngress(s.Ingress) {
		return fmt.Errorf("Invalid value for ingress: %s", s.Ingress)
	}
	return nil
}

func (s *SiteConfigSpec) CheckConsoleIngress() error {
	if !isValidIngress(s.ConsoleIngress) {
		return fmt.Errorf("Invalid value for console-ingress: %s", s.ConsoleIngress)
	}
	return nil
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

type ProxyInitOptions struct {
	Name       string `json:"name,omitempty"`
	StartProxy bool   `json:"startProxy,omitempty`
}

type ProxyBindOptions struct {
	Protocol   string
	Address    string
	Host       string
	Port       string
	ErrIfNoSvc bool
}

type ProxyExposeOptions struct {
	ProxyName string
	Egress    ProxyBindOptions
}

type ProxyEndpoint struct {
	Name      string `json:"name,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      string `json:"port,omitempty"`
	Address   string `json:"address,omitempty"`
	LocalPort string `json:"active,omitempty"`
}

type ProxyInspectResponse struct {
	ProxyName     string
	ProxyUrl      string
	ProxyVersion  string
	TcpConnectors map[string]ProxyEndpoint
	TcpListeners  map[string]ProxyEndpoint
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
	ProxyBind(ctx context.Context, proxyName string, egress ProxyBindOptions) error
	ProxyUnbind(ctx context.Context, proxyName string, address string) error
	ProxyExpose(ctx context.Context, options ProxyExposeOptions) (string, error)
	ProxyUnexpose(ctx context.Context, proxyName string, address string) error
	ProxyForward(ctx context.Context, proxyName string, loopback bool, service *ServiceInterface) error
	ProxyUnforward(ctx context.Context, proxyName string, address string) error
	ProxyInit(ctx context.Context, options ProxyInitOptions) (string, error)
	ProxyDownload(ctx context.Context, proxyName string, downloadPath string) error
	ProxyInspect(ctx context.Context, proxyName string) (*ProxyInspectResponse, error)
	ProxyList(ctx context.Context) ([]*ProxyInspectResponse, error)
	ProxyRemove(ctx context.Context, proxyName string) error
	SiteConfigCreate(ctx context.Context, spec SiteConfigSpec) (*SiteConfig, error)
	SiteConfigUpdate(ctx context.Context, spec SiteConfigSpec) ([]string, error)
	SiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*SiteConfig, error)
	SiteConfigRemove(ctx context.Context) error
	SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) error
	GetNamespace() string
	GetVersion(component string, name string) string
	GetIngressDefault() string
}
