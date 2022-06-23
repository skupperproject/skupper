/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	jsonencoding "encoding/json"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// NamespaceDefault means the VAN is in the  skupper namespace which is applied when not specified by clients
	NamespaceDefault    string = "skupper"
	DefaultVanName      string = "skupper"
	DefaultSiteName     string = "skupper-site"
	ClusterLocalPostfix string = ".svc.cluster.local"
	SiteConfigMapName   string = "skupper-site"
)

// TransportMode describes how a qdr is intended to be deployed, either interior or edge
type TransportMode string

const (
	// TransportModeInterior means the qdr will participate in inter-router protocol exchanges
	TransportModeInterior TransportMode = "interior"
	// TransportModeEdge means that the qdr will connect to interior routers for network access
	TransportModeEdge TransportMode = "edge"
)

// Transport constants
const (
	TransportDeploymentName       string = "skupper-router"
	TransportComponentName        string = "router"
	TransportContainerName        string = "router"
	ConfigSyncContainerName       string = "config-sync"
	TransportLivenessPort         int32  = 9090
	TransportServiceAccountName   string = "skupper-router"
	TransportRoleName             string = "skupper-router"
	TransportRoleBindingName      string = "skupper-router"
	TransportEnvConfig            string = "QDROUTERD_CONF"
	TransportSaslConfig           string = "skupper-sasl-config"
	TransportConfigFile           string = "skrouterd.json"
	TransportConfigMapName        string = "skupper-internal"
	TransportServiceName          string = "skupper-router"
	LocalTransportServiceName     string = "skupper-router-local"
	RouterMaxFrameSizeDefault     int    = 16384
	RouterMaxSessionFramesDefault int    = 640
)

var TransportPolicyRule = []rbacv1.PolicyRule{
	{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"secrets", "pods", "configmaps"},
	},
}

var TransportPrometheusAnnotations = map[string]string{
	"prometheus.io/port":   "9090",
	"prometheus.io/scrape": "true",
}

// Controller constants
const (
	ControllerDeploymentName             string = "skupper-service-controller"
	ControllerComponentName              string = "service-controller"
	ControllerContainerName              string = "service-controller"
	ControllerServiceAccountName         string = "skupper-service-controller"
	ControllerRoleBindingName            string = "skupper-service-controller"
	ControllerClusterRoleBindingNsFormat string = "skupper-service-controller-%s"
	ControllerRoleName                   string = "skupper-service-controller"
	ControllerClusterRoleName            string = "skupper-service-controller"
	ControllerConfigPath                 string = "/etc/messaging/"
	ControllerServiceName                string = "skupper"
)

var ControllerPolicyRule = []rbacv1.PolicyRule{
	{
		Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		APIGroups: []string{""},
		Resources: []string{"services", "configmaps", "pods", "secrets"},
	},
	{
		Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		APIGroups: []string{"apps"},
		Resources: []string{"deployments", "statefulsets"},
	},
	{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{"apps"},
		Resources: []string{"daemonsets"},
	},
	{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{"route.openshift.io"},
		Resources: []string{"routes"},
	},
	{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{"networking.k8s.io"},
		Resources: []string{"ingresses"},
	},
}

// Certifcates/Secrets constants
const (
	LocalClientSecret        string = "skupper-local-client"
	LocalServerSecret        string = "skupper-local-server"
	LocalCaSecret            string = "skupper-local-ca"
	SiteServerSecret         string = "skupper-site-server"
	ClaimsServerSecret       string = "skupper-claims-server"
	SiteCaSecret             string = "skupper-site-ca"
	ConsoleServerSecret      string = "skupper-console-certs"
	OauthRouterConsoleSecret string = "skupper-router-console-certs"
	ServiceCaSecret          string = "skupper-service-ca"
	ServiceClientSecret      string = "skupper-service-client" //Secret that is used in sslProfiles for all http2 connectors with tls enabled
)

// Skupper qualifiers
const (
	BaseQualifier               string = "skupper.io"
	InternalQualifier           string = "internal." + BaseQualifier
	AddressQualifier            string = BaseQualifier + "/address"
	PortQualifier               string = BaseQualifier + "/port"
	ProxyQualifier              string = BaseQualifier + "/proxy"
	TargetServiceQualifier      string = BaseQualifier + "/target"
	ControlledQualifier         string = InternalQualifier + "/controlled"
	ServiceQualifier            string = InternalQualifier + "/service"
	OriginQualifier             string = InternalQualifier + "/origin"
	OriginalSelectorQualifier   string = InternalQualifier + "/originalSelector"
	OriginalTargetPortQualifier string = InternalQualifier + "/originalTargetPort"
	OriginalAssignedQualifier   string = InternalQualifier + "/originalAssignedPort"
	InternalTypeQualifier       string = InternalQualifier + "/type"
	SkupperTypeQualifier        string = BaseQualifier + "/type"
	TypeProxyQualifier          string = InternalTypeQualifier + "=proxy"
	SkupperDisabledQualifier    string = InternalQualifier + "/disabled"
	TypeToken                   string = "connection-token"
	TypeClaimRecord             string = "token-claim-record"
	TypeClaimRequest            string = "token-claim"
	TypeGatewayToken            string = "gateway-connection-token"
	TypeTokenQualifier          string = BaseQualifier + "/type=connection-token"
	TypeTokenRequestQualifier   string = BaseQualifier + "/type=connection-token-request"
	TokenGeneratedBy            string = BaseQualifier + "/generated-by"
	SiteVersion                 string = BaseQualifier + "/site-version"
	TokenCost                   string = BaseQualifier + "/cost"
	UpdatedAnnotation           string = InternalQualifier + "/updated"
	AnnotationExcludes          string = BaseQualifier + "/exclude-annotations"
	LabelExcludes               string = BaseQualifier + "/exclude-labels"
	ServiceLabels               string = BaseQualifier + "/service-labels"
	ComponentAnnotation         string = BaseQualifier + "/component"
	SiteControllerIgnore        string = InternalQualifier + "/site-controller-ignore"
	RouterComponent             string = "router"
	ClaimExpiration             string = BaseQualifier + "/claim-expiration"
	ClaimsRemaining             string = BaseQualifier + "/claims-remaining"
	ClaimsMade                  string = BaseQualifier + "/claims-made"
	ClaimUrlAnnotationKey       string = BaseQualifier + "/url"
	ClaimPasswordDataKey        string = "password"
	ClaimCaCertDataKey          string = "ca.crt"
	ClaimRequestSelector        string = SkupperTypeQualifier + "=" + TypeClaimRequest
	LastFailedAnnotationKey     string = InternalQualifier + "/last-failed"
	StatusAnnotationKey         string = InternalQualifier + "/status"
	GatewayQualifier            string = InternalQualifier + "/gateway"
)

// standard labels
const (
	AppLabel    string = "app.kubernetes.io/name"
	PartOfLabel string = "app.kubernetes.io/part-of"
	AppName     string = "skupper"
)

// Service Interface constants
const (
	ServiceInterfaceConfigMap string = "skupper-services"
)

// OpenShift constants
const (
	OpenShiftServingCertSecretName string = "service.alpha.openshift.io/serving-cert-secret-name"
)

// Console constants
const (
	ConsolePortName                        string = "console"
	ConsoleDefaultServicePort              int32  = 8080
	ConsoleDefaultServiceTargetPort        int32  = 8080
	ConsoleOpenShiftServicePort            int32  = 8888
	ConsoleOpenShiftOauthServicePort       int32  = 443
	ConsoleOpenShiftOauthServiceTargetPort int32  = 8443
	ConsoleRouteName                       string = "skupper"
	RouterConsoleRouteName                 string = "skupper-router-console"
	RouterConsoleServiceName               string = "skupper-router-console"
)

type ConsoleAuthMode string

const (
	ConsoleAuthModeOpenshift ConsoleAuthMode = "openshift"
	ConsoleAuthModeInternal                  = "internal"
	ConsoleAuthModeUnsecured                 = "unsecured"
)

const (
	ClaimRedemptionPort      int32  = 8081
	ClaimRedemptionPortName  string = "claims"
	ClaimRedemptionRouteName string = "claims"
)

// Assembly constants
const (
	AmqpDefaultPort         int32  = 5672
	AmqpsDefaultPort        int32  = 5671
	EdgeRole                string = "edge"
	EdgeRouteName           string = "skupper-edge"
	EdgeListenerPort        int32  = 45671
	InterRouterRole         string = "inter-router"
	InterRouterListenerPort int32  = 55671
	InterRouterRouteName    string = "skupper-inter-router"
	InterRouterProfile      string = "skupper-internal"
	IngressName             string = "skupper"
)

// Service Sync constants
const (
	ServiceSyncAddress = "mc/$skupper-service-sync"
)

const (
	SkupperServiceCertPrefix string = "skupper-tls-"
)

// RouterSpec is the specification of VAN network with router, controller and assembly
type RouterSpec struct {
	Name                  string          `json:"name,omitempty"`
	Namespace             string          `json:"namespace,omitempty"`
	AuthMode              ConsoleAuthMode `json:"authMode,omitempty"`
	Transport             DeploymentSpec  `json:"transport,omitempty"`
	ConfigSync            DeploymentSpec  `json:"configSync,omitempty"`
	Controller            DeploymentSpec  `json:"controller,omitempty"`
	RouterConfig          string          `json:"routerConfig,omitempty"`
	Users                 []User          `json:"users,omitempty"`
	CertAuthoritys        []CertAuthority `json:"certAuthoritys,omitempty"`
	TransportCredentials  []Credential    `json:"transportCredentials,omitempty"`
	ControllerCredentials []Credential    `json:"controllerCredentials,omitempty"`
}

type ImageDetails struct {
	Name       string `json:"image,omitempty"`
	PullPolicy string `json:"imagePullPolicy,omitempty"`
}

// DeploymentSpec for the VAN router or controller components to run within a cluster
type DeploymentSpec struct {
	Image               ImageDetails                 `json:"image,omitempty"`
	Replicas            int32                        `json:"replicas,omitempty"`
	LivenessPort        int32                        `json:"livenessPort,omitempty"`
	Labels              map[string]string            `json:"labels,omitempty"`
	Annotations         map[string]string            `json:"annotations,omitempty"`
	LabelSelector       map[string]string            `json:"labelSelector,omitempty"`
	EnvVar              []corev1.EnvVar              `json:"envVar,omitempty"`
	Ports               []corev1.ContainerPort       `json:"ports,omitempty"`
	Volumes             []corev1.Volume              `json:"volumes,omitempty"`
	VolumeMounts        [][]corev1.VolumeMount       `json:"volumeMounts,omitempty"`
	Roles               []*rbacv1.Role               `json:"roles,omitempty"`
	RoleBindings        []*rbacv1.RoleBinding        `json:"roleBinding,omitempty"`
	ClusterRoles        []*rbacv1.ClusterRole        `json:"clusterRoles,omitempty"`
	ClusterRoleBindings []*rbacv1.ClusterRoleBinding `json:"clusterRoleBinding,omitempty"`
	Routes              []*routev1.Route             `json:"routes,omitempty"`
	ServiceAccounts     []*corev1.ServiceAccount     `json:"serviceAccounts,omitempty"`
	Services            []*corev1.Service            `json:"services,omitempty"`
	Sidecars            []*corev1.Container          `json:"sidecars,omitempty"`
	Affinity            map[string]string            `json:"affinity,omitempty"`
	AntiAffinity        map[string]string            `json:"antiAffinity,omitempty"`
	NodeSelector        map[string]string            `json:"nodeSelector,omitempty"`
	CpuRequest          *resource.Quantity           `json:"cpuRequest,omitempty"`
	MemoryRequest       *resource.Quantity           `json:"memoryRequest,omitempty"`
	CpuLimit            *resource.Quantity           `json:"cpuLimit,omitempty"`
	MemoryLimit         *resource.Quantity           `json:"memoryLimit,omitempty"`
}

func (s *DeploymentSpec) GetCpuRequest() resource.Quantity {
	if s.CpuRequest == nil {
		return resource.Quantity{}
	}
	return *s.CpuRequest
}

func (s *DeploymentSpec) GetMemoryRequest() resource.Quantity {
	if s.MemoryRequest == nil {
		return resource.Quantity{}
	}
	return *s.MemoryRequest
}

func (s *DeploymentSpec) GetCpuLimit() resource.Quantity {
	if s.CpuLimit == nil {
		return s.GetCpuRequest()
	}
	return *s.CpuLimit
}

func (s *DeploymentSpec) GetMemoryLimit() resource.Quantity {
	if s.MemoryLimit == nil {
		return s.GetMemoryRequest()
	}
	return *s.MemoryLimit
}

func (s *DeploymentSpec) HasCpuRequest() bool {
	return s.CpuRequest != nil
}

func (s *DeploymentSpec) HasMemoryRequest() bool {
	return s.MemoryRequest != nil
}

func (s *DeploymentSpec) HasCpuLimit() bool {
	return s.CpuLimit != nil || s.HasCpuRequest()
}

func (s *DeploymentSpec) HasMemoryLimit() bool {
	return s.MemoryLimit != nil || s.HasMemoryRequest()
}

type Resources interface {
	GetCpuRequest() resource.Quantity
	GetMemoryRequest() resource.Quantity
	GetCpuLimit() resource.Quantity
	GetMemoryLimit() resource.Quantity
	HasCpuRequest() bool
	HasMemoryRequest() bool
	HasCpuLimit() bool
	HasMemoryLimit() bool
}

// AssemblySpec for the links and connectors that form the VAN topology
type AssemblySpec struct {
	Name                  string       `json:"name,omitempty"`
	Mode                  string       `json:"mode,omitempty"`
	Listeners             []Listener   `json:"listeners,omitempty"`
	InterRouterListeners  []Listener   `json:"interRouterListeners,omitempty"`
	EdgeListeners         []Listener   `json:"edgeListeners,omitempty"`
	SslProfiles           []SslProfile `json:"sslProfiles,omitempty"`
	Connectors            []Connector  `json:"connectors,omitempty"`
	InterRouterConnectors []Connector  `json:"interRouterConnectors,omitempty"`
	EdgeConnectors        []Connector  `json:"edgeConnectors,omitempty"`
}

type RouterStatusSpec struct {
	SiteName               string                  `json:"siteName,omitempty"`
	Mode                   string                  `json:"mode,omitempty"`
	TransportReadyReplicas int32                   `json:"transportReadyReplicas,omitempty"`
	ConnectedSites         TransportConnectedSites `json:"connectedSites,omitempty"`
	BindingsCount          int                     `json:"bindingsCount,omitempty"`
}

type Listener struct {
	Name             string `json:"name,omitempty"`
	Host             string `json:"host,omitempty"`
	Port             int32  `json:"port"`
	RouteContainer   bool   `json:"routeContainer,omitempty"`
	Http             bool   `json:"http,omitempty"`
	Cost             int32  `json:"cost,omitempty"`
	SslProfile       string `json:"sslProfile,omitempty"`
	SaslMechanisms   string `json:"saslMechanisms,omitempty"`
	AuthenticatePeer bool   `json:"authenticatePeer,omitempty"`
	LinkCapacity     int32  `json:"linkCapacity,omitempty"`
}

type SslProfile struct {
	Name   string `json:"name,omitempty"`
	Cert   string `json:"cert,omitempty"`
	Key    string `json:"key,omitempty"`
	CaCert string `json:"caCert,omitempty"`
}

type ConnectorRole string

const (
	ConnectorRoleInterRouter ConnectorRole = "inter-router"
	ConnectorRoleEdge                      = "edge"
)

const (
	ConsoleIngressPrefix     = "skupper-console"
	ClaimsIngressPrefix      = "skupper-claims"
	InterRouterIngressPrefix = "skupper-inter-router"
	EdgeIngressPrefix        = "skupper-edge"
)

type Connector struct {
	Name           string `json:"name,omitempty"`
	Role           string `json:"role,omitempty"`
	Host           string `json:"host"`
	Port           string `json:"port"`
	RouteContainer bool   `json:"routeContainer,omitempty"`
	Cost           int32  `json:"cost,omitempty"`
	VerifyHostname bool   `json:"verifyHostname,omitempty"`
	SslProfile     string `json:"sslProfile,omitempty"`
	LinkCapacity   int32  `json:"linkCapacity,omitempty"`
}

type Credential struct {
	CA          string
	Name        string
	Subject     string
	Hosts       []string
	ConnectJson bool
	Post        bool
	Data        map[string][]byte
	Simple      bool `default:"false"`
}

type CertAuthority struct {
	Name string
}

type User struct {
	Name     string
	Password string
}

type TransportConnectedSites struct {
	Direct   int
	Indirect int
	Total    int
	Warnings []string
}

type ServiceInterface struct {
	Address        string                   `json:"address" yaml:"address"`
	Protocol       string                   `json:"protocol" yaml:"protocol"`
	Ports          []int                    `json:"ports" yaml:"ports"`
	EventChannel   bool                     `json:"eventchannel,omitempty" yaml:"eventchannel,omitempty"`
	Aggregate      string                   `json:"aggregate,omitempty" yaml:"aggregate,omitempty"`
	Headless       *Headless                `json:"headless,omitempty" yaml:"headless,omitempty"`
	Labels         map[string]string        `json:"labels,omitempty" yaml:"labels,omitempty"`
	Targets        []ServiceInterfaceTarget `json:"targets" yaml:"targets,omitempty"`
	Origin         string                   `json:"origin,omitempty" yaml:"origin,omitempty"`
	EnableTls      bool                     `json:"enableTls,omitempty"`
	TlsCredentials string                   `json:"tlsCredentials,omitempty"`
}

type ServiceInterfaceList []ServiceInterface

func (sl *ServiceInterfaceList) ConvertFrom(updates string) error {
	err := jsonencoding.Unmarshal([]byte(updates), sl)
	portsDefined := len(*sl) == 0 || len((*sl)[0].Ports) > 0
	if err == nil && portsDefined {
		return nil
	}
	// Try converting from V1 format
	v1ServiceList := []ServiceInterfaceV1{}
	err = jsonencoding.Unmarshal([]byte(updates), &v1ServiceList)
	if err != nil {
		return err
	}
	// Converting from V1
	for i, v1Svc := range v1ServiceList {
		svc := &(*sl)[i]
		svc.Ports = []int{v1Svc.Port}
		// Converting Headless
		if v1Svc.Headless != nil {
			v1hl := v1Svc.Headless
			tPort := v1Svc.Port
			if v1hl.TargetPort > 0 {
				tPort = v1hl.TargetPort
			}
			svc.Headless.TargetPorts = map[int]int{
				v1Svc.Port: tPort,
			}
		}
		// Converting Targets
		if len(v1Svc.Targets) > 0 {
			for i, v1Target := range v1Svc.Targets {
				tPort := v1Svc.Port
				if v1Target.TargetPort > 0 {
					tPort = v1Target.TargetPort
				}
				svc.Targets[i].TargetPorts = map[int]int{
					v1Svc.Port: tPort,
				}
			}
		}
	}
	return nil
}

type ServiceInterfaceTarget struct {
	Name        string      `json:"name,omitempty"`
	Selector    string      `json:"selector,omitempty"`
	TargetPorts map[int]int `json:"targetPorts,omitempty"`
	Service     string      `json:"service,omitempty"`
}

type ServiceInterfaceV1 struct {
	Address      string                     `json:"address" yaml:"address"`
	Protocol     string                     `json:"protocol" yaml:"protocol"`
	Port         int                        `json:"port" yaml:"port"`
	EventChannel bool                       `json:"eventchannel,omitempty" yaml:"eventchannel,omitempty"`
	Aggregate    string                     `json:"aggregate,omitempty" yaml:"aggregate,omitempty"`
	Headless     *HeadlessV1                `json:"headless,omitempty" yaml:"headless,omitempty"`
	Labels       map[string]string          `json:"labels,omitempty" yaml:"labels,omitempty"`
	Targets      []ServiceInterfaceTargetV1 `json:"targets" yaml:"targets,omitempty"`
	Origin       string                     `json:"origin,omitempty" yaml:"origin,omitempty"`
}

type ServiceInterfaceTargetV1 struct {
	Name       string `json:"name,omitempty"`
	Selector   string `json:"selector,omitempty"`
	TargetPort int    `json:"targetPort,omitempty"`
	Service    string `json:"service,omitempty"`
}

func (service *ServiceInterface) AddTarget(target *ServiceInterfaceTarget) {
	modified := false
	targets := []ServiceInterfaceTarget{}
	for _, t := range service.Targets {
		if t.Name == target.Name {
			modified = true
			targets = append(targets, *target)
		} else {
			targets = append(targets, t)
		}
	}
	if !modified {
		targets = append(targets, *target)
	}
	service.Targets = targets
}

type Headless struct {
	Name          string             `json:"name" yaml:"name"`
	Size          int                `json:"size" yaml:"size"`
	TargetPorts   map[int]int        `json:"targetPorts,omitempty" yaml:"targetPorts,omitempty"`
	Affinity      map[string]string  `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	AntiAffinity  map[string]string  `json:"antiAffinity,omitempty" yaml:"antiAffinity,omitempty"`
	NodeSelector  map[string]string  `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	CpuRequest    *resource.Quantity `json:"cpuRequest,omitempty" yaml:"cpuRequest,omitempty"`
	MemoryRequest *resource.Quantity `json:"memoryRequest,omitempty" yaml:"memoryRequest,omitempty"`
	CpuLimit      *resource.Quantity `json:"cpuLimit,omitempty" yaml:"cpuLimit,omitempty"`
	MemoryLimit   *resource.Quantity `json:"memoryLimit,omitempty" yaml:"memoryLimit,omitempty"`
}

func (s *Headless) GetCpuRequest() resource.Quantity {
	if s.CpuRequest == nil {
		return resource.Quantity{}
	}
	return *s.CpuRequest
}

func (s *Headless) GetMemoryRequest() resource.Quantity {
	if s.MemoryRequest == nil {
		return resource.Quantity{}
	}
	return *s.MemoryRequest
}

func (s *Headless) GetCpuLimit() resource.Quantity {
	if s.CpuLimit == nil {
		return s.GetCpuRequest()
	}
	return *s.CpuLimit
}

func (s *Headless) GetMemoryLimit() resource.Quantity {
	if s.MemoryLimit == nil {
		return s.GetMemoryRequest()
	}
	return *s.MemoryLimit
}

func (s *Headless) HasCpuRequest() bool {
	return s.CpuRequest != nil
}

func (s *Headless) HasMemoryRequest() bool {
	return s.MemoryRequest != nil
}

func (s *Headless) HasCpuLimit() bool {
	return s.CpuLimit != nil || s.HasCpuRequest()
}

func (s *Headless) HasMemoryLimit() bool {
	return s.MemoryLimit != nil || s.HasMemoryRequest()
}

type HeadlessV1 struct {
	Name          string             `json:"name"`
	Size          int                `json:"size"`
	TargetPort    int                `json:"targetPort,omitempty"`
	Affinity      map[string]string  `json:"affinity,omitempty"`
	AntiAffinity  map[string]string  `json:"antiAffinity,omitempty"`
	NodeSelector  map[string]string  `json:"nodeSelector,omitempty"`
	CpuRequest    *resource.Quantity `json:"cpuRequest,omitempty"`
	MemoryRequest *resource.Quantity `json:"memoryRequest,omitempty"`
}

type ByServiceInterfaceAddress []ServiceInterface

func (a ByServiceInterfaceAddress) Len() int {
	return len(a)
}

func (a ByServiceInterfaceAddress) Less(i, j int) bool {
	return a[i].Address > a[i].Address
}

func (a ByServiceInterfaceAddress) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func QualifiedServiceName(name string, namespace string) string {
	return name + "." + namespace + ".svc.cluster.local"
}
