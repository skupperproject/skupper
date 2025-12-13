package types

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/internal/network"
	skupperclient "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned"

	openshiftroute "github.com/openshift/client-go/route/clientset/versioned"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	ENV_PLATFORM           = "SKUPPER_PLATFORM"
	ENV_SYSTEM_AUTO_RELOAD = "SKUPPER_SYSTEM_RELOAD_TYPE"

	SystemReloadTypeAuto   string = "auto"
	SystemReloadTypeManual string = "manual"
)

type ConnectorCreateOptions struct {
	SkupperNamespace string
	Name             string
	Cost             int32
	Secret           *corev1.Secret
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
	Created     string
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
	CpuLimit     string
	MemoryLimit  string
}

type RouterOptions struct {
	Tuning
	Logging             []RouterLogConfig
	MaxFrameSize        int
	MaxSessionFrames    int
	DataConnectionCount string
	IngressHost         string
	ServiceAnnotations  map[string]string
	PodAnnotations      map[string]string
	LoadBalancerIp      string
	DisableMutualTLS    bool
}

type ControllerOptions struct {
	Tuning
	IngressHost        string
	ServiceAnnotations map[string]string
	PodAnnotations     map[string]string
	LoadBalancerIp     string
}

type ConfigSyncOptions struct {
	Tuning
}

type FlowCollectorOptions struct {
	Tuning
	FlowRecordTtl time.Duration
}

type PrometheusServerOptions struct {
	Tuning
	ExternalServer string
	AuthMode       string
	User           string
	Password       string
	PodAnnotations map[string]string
}

type SiteConfigSpec struct {
	SkupperName              string
	SkupperNamespace         string
	RouterMode               string
	Routers                  int
	EnableController         bool
	EnableServiceSync        bool
	SiteTtl                  time.Duration
	EnableConsole            bool
	EnableFlowCollector      bool
	EnableRestAPI            bool
	AuthMode                 string
	User                     string
	Password                 string
	Ingress                  string
	IngressAnnotations       map[string]string
	ConsoleIngress           string
	IngressHost              string
	Replicas                 int32
	SiteControlled           bool
	CreateNetworkPolicy      bool
	Annotations              map[string]string
	Labels                   map[string]string
	Router                   RouterOptions
	Controller               ControllerOptions
	ConfigSync               ConfigSyncOptions
	FlowCollector            FlowCollectorOptions
	PrometheusServer         PrometheusServerOptions
	Platform                 Platform
	RunAsUser                int64
	RunAsGroup               int64
	EnableClusterPermissions bool
	EnableSkupperEvents      bool
}

const (
	IngressRouteString            string = "route"
	IngressLoadBalancerString     string = "loadbalancer"
	IngressNodePortString         string = "nodeport"
	IngressNginxIngressString     string = "nginx-ingress-v1"
	IngressContourHttpProxyString string = "contour-http-proxy"
	IngressKubernetes             string = "ingress"
	IngressPodmanExternal         string = "external"
	IngressNoneString             string = "none"
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
func (s *SiteConfigSpec) IsIngressNginxIngress() bool {
	return s.Ingress == IngressNginxIngressString
}
func (s *SiteConfigSpec) IsIngressContourHttpProxy() bool {
	return s.Ingress == IngressContourHttpProxyString
}
func (s *SiteConfigSpec) IsIngressKubernetes() bool {
	return s.Ingress == IngressKubernetes
}
func (s *SiteConfigSpec) IsIngressPodmanHost() bool {
	return s.Ingress == IngressPodmanExternal
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
func (s *SiteConfigSpec) IsConsoleIngressNginxIngress() bool {
	return s.getConsoleIngress() == IngressNginxIngressString
}
func (s *SiteConfigSpec) IsConsoleIngressContourHttpProxy() bool {
	return s.getConsoleIngress() == IngressContourHttpProxyString
}
func (s *SiteConfigSpec) IsConsoleIngressKubernetes() bool {
	return s.getConsoleIngress() == IngressKubernetes
}
func (s *SiteConfigSpec) IsConsoleIngressNone() bool {
	return s.getConsoleIngress() == IngressNoneString
}
func (s *SiteConfigSpec) IsEdge() bool {
	return s.RouterMode == string(TransportModeEdge)
}
func (s *SiteConfigSpec) getConsoleIngress() string {
	if s.ConsoleIngress == "" {
		return s.Ingress
	}
	return s.ConsoleIngress
}

func ValidIngressOptions(platform Platform) []string {
	switch platform {
	case PlatformPodman:
		return []string{IngressPodmanExternal, IngressNoneString}
	default:
		return []string{IngressRouteString, IngressLoadBalancerString, IngressNodePortString, IngressNginxIngressString, IngressContourHttpProxyString, IngressKubernetes, IngressNoneString}
	}
}

func ValidAuthOptions(platform Platform) []string {
	switch platform {
	case PlatformPodman:
		return []string{"internal", "unsecured"}
	default:
		return []string{"internal", "unsecured", "openshift"}
	}
}

func isValidIngress(platform Platform, ingress string) bool {
	if ingress == "" {
		return true
	}
	for _, value := range ValidIngressOptions(platform) {
		if ingress == value {
			return true
		}
	}
	return false
}

func (s *SiteConfigSpec) CheckIngress() error {
	if !isValidIngress(s.Platform, s.Ingress) {
		return fmt.Errorf("Invalid value for ingress: %s", s.Ingress)
	}
	return nil
}

func (s *SiteConfigSpec) CheckConsoleIngress() error {
	if !isValidIngress(s.Platform, s.ConsoleIngress) {
		return fmt.Errorf("Invalid value for console-ingress: %s", s.ConsoleIngress)
	}
	return nil
}

func (s *SiteConfigSpec) GetRouterIngressHost() string {
	if s.Router.IngressHost != "" {
		return s.Router.IngressHost
	}
	return s.IngressHost
}

func (s *SiteConfigSpec) GetControllerIngressHost() string {
	if s.Controller.IngressHost != "" {
		return s.Controller.IngressHost
	}
	return s.IngressHost
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

type GatewayEndpoint struct {
	Name        string           `json:"name,omitempty" yaml:"name,omitempty"`
	Host        string           `json:"host,omitempty" yaml:"host,omitempty"`
	Loopback    bool             `json:"loopback,omitempty" yaml:"loopback,omitempty"`
	LocalPort   string           `json:"localPort,omitempty" yaml:"local_port,omitempty"`
	Service     ServiceInterface `json:"service,omitempty" yaml:"service,omitempty"`
	TargetPorts []int            `json:"targetPorts,omitempty" yaml:"target_ports,omitempty"`
}

type GatewayInspectResponse struct {
	Name       string
	Type       string
	Url        string
	Version    string
	Connectors map[string]GatewayEndpoint
	Listeners  map[string]GatewayEndpoint
}

type VanClientInterface interface {
	RouterCreate(ctx context.Context, options SiteConfig) error
	RouterInspect(ctx context.Context) (*RouterInspectResponse, error)
	RouterInspectNamespace(ctx context.Context, namespace string) (*RouterInspectResponse, error)
	RouterRemove(ctx context.Context) error
	RouterUpdateVersion(ctx context.Context, hup bool) (bool, error)
	RouterUpdateVersionInNamespace(ctx context.Context, hup bool, namespace string) (bool, error)
	ConnectorCreateFromFile(ctx context.Context, secretFile string, options ConnectorCreateOptions) (*corev1.Secret, error)
	ConnectorCreateSecretFromData(ctx context.Context, options ConnectorCreateOptions) (*corev1.Secret, error)
	ConnectorCreate(ctx context.Context, secret *corev1.Secret, options ConnectorCreateOptions) error
	ConnectorInspect(ctx context.Context, name string) (*LinkStatus, error)
	ConnectorList(ctx context.Context) ([]LinkStatus, error)
	ConnectorRemove(ctx context.Context, options ConnectorRemoveOptions) error
	ConnectorTokenCreateFromTemplate(ctx context.Context, tokenName string, templateName string) (*corev1.Secret, bool, error)
	ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error)
	ConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error
	TokenClaimCreate(ctx context.Context, name string, password []byte, expiry time.Duration, uses int) (*corev1.Secret, bool, error)
	TokenClaimCreateFile(ctx context.Context, name string, password []byte, expiry time.Duration, uses int, secretFile string) error
	ServiceInterfaceCreate(ctx context.Context, service *ServiceInterface) error
	ServiceInterfaceInspect(ctx context.Context, address string) (*ServiceInterface, error)
	ServiceInterfaceList(ctx context.Context) ([]*ServiceInterface, error)
	ServiceInterfaceRemove(ctx context.Context, address string) error
	ServiceInterfaceUpdate(ctx context.Context, service *ServiceInterface) error
	ServiceInterfaceBind(ctx context.Context, service *ServiceInterface, targetType string, targetName string, targetPorts map[int]int, namespace string) error
	GetHeadlessServiceConfiguration(targetName string, protocol string, address string, ports []int, publishNotReadyAddresses bool, namespace string) (*ServiceInterface, error)
	ServiceInterfaceUnbind(ctx context.Context, targetType string, targetName string, address string, deleteIfNoTargets bool, namespace string) error
	GatewayBind(ctx context.Context, gatewayName string, endpoint GatewayEndpoint) error
	GatewayUnbind(ctx context.Context, gatewayName string, endpoint GatewayEndpoint) error
	GatewayExpose(ctx context.Context, gatewayName string, gatewayType string, endpoint GatewayEndpoint) (string, error)
	GatewayUnexpose(ctx context.Context, gatewayName string, endpoint GatewayEndpoint, deleteLast bool) error
	GatewayForward(ctx context.Context, gatewayName string, endpoint GatewayEndpoint) error
	GatewayUnforward(ctx context.Context, gatewayName string, endpoint GatewayEndpoint) error
	GatewayInit(ctx context.Context, gatewayName string, gatewayType string, configFile string) (string, error)
	GatewayDownload(ctx context.Context, gatewayName string, downloadPath string) (string, error)
	GatewayExportConfig(ctx context.Context, targetGatewayName string, exportGatewayName string, exportPath string) (string, error)
	GatewayGenerateBundle(ctx context.Context, configFile string, bundlePath string) (string, error)
	GatewayInspect(ctx context.Context, gatewayName string) (*GatewayInspectResponse, error)
	GatewayList(ctx context.Context) ([]*GatewayInspectResponse, error)
	GatewayRemove(ctx context.Context, gatewayName string) error
	SiteConfigCreate(ctx context.Context, spec SiteConfigSpec) (*SiteConfig, error)
	SiteConfigUpdate(ctx context.Context, spec SiteConfigSpec) ([]string, error)
	SiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*SiteConfig, error)
	SiteConfigRemove(ctx context.Context) error
	SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) (string, error)
	SkupperEvents(verbose bool) (*bytes.Buffer, error)
	SkupperCheckService(service string, verbose bool) (*bytes.Buffer, error)
	SkupperPolicies(verbose bool) (*bytes.Buffer, error)
	GetNamespace() string
	GetVersion(component string, name string) string
	GetIngressDefault() string
	RevokeAccess(ctx context.Context) error
	NetworkStatus(ctx context.Context) (*network.NetworkStatusInfo, error)
	GetConsoleUrl(namespace string) (string, error)
	GetKubeClient() kubernetes.Interface
	GetDynamicClient() dynamic.Interface
	GetDiscoveryClient() discovery.DiscoveryInterface
	GetRouteClient() routev1client.RouteV1Interface
	GetRouteInterface() openshiftroute.Interface
	GetSkupperClient() skupperclient.Interface
}
