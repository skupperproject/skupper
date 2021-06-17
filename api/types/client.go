package types

import (
	"context"
	"fmt"
	"time"

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

type LinkStatus struct {
	Name        string
	Url         string
	Cost        int
	Connected   bool
	Configured  bool
	Description string
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
	IngressHost      string
}

type ControllerOptions struct {
	Tuning
	IngressHost string
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
	IngressNodePortString     string = "nodeport"
	IngressNoneString         string = "none"
)

func (s *SiteConfigSpec) IsIngressRoute() bool {
	return s.Ingress == IngressRouteString
}
func (s *SiteConfigSpec) IsIngressLoadBalancer() bool {
	return s.Ingress == IngressLoadBalancerString
}
func (s *SiteConfigSpec) IsIngressNodePort() bool {
	return s.Ingress == IngressNodePortString
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
func (s *SiteConfigSpec) IsConsoleIngressNodePort() bool {
	return s.getConsoleIngress() == IngressNodePortString
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
	return ingress == "" || ingress == IngressRouteString || ingress == IngressLoadBalancerString || ingress == IngressNodePortString || ingress == IngressNoneString
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
	ConnectorInspect(ctx context.Context, name string) (*LinkStatus, error)
	ConnectorList(ctx context.Context) ([]LinkStatus, error)
	ConnectorRemove(ctx context.Context, options ConnectorRemoveOptions) error
	ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error)
	ConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error
	TokenClaimCreate(ctx context.Context, name string, password []byte, expiry time.Duration, uses int, secretFile string) error
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
	SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) (string, error)
	GetNamespace() string
	GetVersion(component string, name string) string
	GetIngressDefault() string
	RevokeAccess(ctx context.Context) error
}
