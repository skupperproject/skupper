package common

import (
	"time"
)

const (
	FlagNamePlatform   = "platform"
	FlagDescPlatform   = "Set the platform type to use [kubernetes, podman, docker, systemd]"
	FlagNameNamespace  = "namespace"
	FlagDescNamespace  = "Set the namespace"
	FlagNameContext    = "context"
	FlagDescContext    = "Set the kubeconfig context"
	FlagNameKubeconfig = "kubeconfig"
	FlagDescKubeconfig = "Path to the kubeconfig file to use"

	FlagNameEnableLinkAccess = "enable-link-access"
	FlagDescEnableLinkAccess = "allow access for incoming links from remote sites (default: false)"
	FlagNameLinkAccessType   = "link-access-type"
	FlagDescLinkAccessType   = `configure external access for links from remote sites.
Choices: [route|loadbalancer]. Default: On OpenShift, route is the default; 
for other Kubernetes flavors, loadbalancer is the default.`
	FlagNameOutput                  = "output"
	FlagDescOutput                  = "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml"
	FlagVerboseOutput               = "print verbose output to the console. Choices: json, yaml"
	FlagNameServiceAccount          = "service-account"
	FlagDescServiceAccount          = "the Kubernetes service account under which to run the Skupper controller"
	FlagNameBindHost                = "bind-host"
	FlagDescBindHost                = "A valid host or ip that can be used to bind a local port"
	FlagNameSubjectAlternativeNames = "subject-alternative-names"
	FlagDescSubjectAlternativeNames = "Add subject alternative names for the router access in non kubernetes environments"

	FlagNameTlsCredentials     = "tls-credentials"
	FlagDescTlsCredentials     = "the name of a Kubernetes secret containing the generated or externally-supplied TLS credentials."
	FlagNameCost               = "cost"
	FlagDescCost               = "the configured \"expense\" of sending traffic over the link."
	FlagNameGenerateCredential = "generate-credential"
	FlagDescGenerateCredential = "generate the necessary credentials to create the link"
	FlagNameTimeout            = "timeout"
	FlagDescTimeout            = "raise an error if the operation does not complete in the given period of time (expressed in seconds)."

	FlagNameRedemptionsAllowed = "redemptions-allowed"
	FlagDescRedemptionsAllowed = "The number of times an access token for this grant can be redeemed."
	FlagNameExpirationWindow   = "expiration-window"
	FlagDescExpirationWindow   = "The period of time in which an access token for this grant can be redeemed."
	FlagNameToken              = "name"
	FlagDescToken              = "The name of token issued"

	FlagNameRoutingKey          = "routing-key"
	FlagDescRoutingKey          = "The identifier used to route traffic from listeners to connectors"
	FlagNameHost                = "host"
	FlagDescHost                = "The hostname or IP address of the local connector"
	FlagNameConnectorType       = "type"
	FlagDescConnectorType       = "The connector type. Choices: [tcp]."
	FlagNameIncludeNotReadyPods = "include-not-ready"
	FlagDescIncludeNotRead      = "If true, include server pods that are not in the ready state."
	FlagNameSelector            = "selector"
	FlagDescSelector            = "A Kubernetes label selector for specifying target server pods."
	FlagNameWorkload            = "workload"
	FlagDescWorkload            = "A Kubernetes resource name that identifies a workload expressed like resource-type/resource-name. Expected resource types: services, daemonsets, deployments, and statefulsets."

	FlagNameConnectorPort = "port"
	FlagDescConnectorPort = "The port of the local connector"

	FlagNameConnectorStatusOutput = "output"
	FlagDescConnectorStatusOutput = "print status of connectors Choices: json, yaml"

	FlagNameListenerType = "type"
	FlagDescListenerType = "The listener type. Choices: [tcp]."
	FlagNameListenerPort = "port"
	FlagDescListenerPort = "The port of the local listener"
	FlagNameListenerHost = "host"
	FlagDescListenerHost = "The hostname or IP address of the local listener. Clients at this site use the listener host and port to establish connections to the remote service."

	FlagNamePath     = "path"
	FlagDescPath     = "Custom resources location on the file system"
	FlagNameStrategy = "strategy"
	FlagDescStrategy = "The bundle strategy to be produced. Choices: bundle, tarball"
	FlagNameForce    = "force"
	FlagDescForce    = "Forces to overwrite an existing namespace"

	FlagNameWait       = "wait"
	FlagDescWait       = "Wait for the given status before exiting. Choices: configured, ready, none"
	FlagDescDeleteWait = "Wait for deletion to complete before exiting"

	FlagNameAll       = "all"
	FlagDescAll       = "delete all skupper resources in current namespace"
	FlagDescDeleteAll = "delete all skupper resources associated with site in current namespace"
)

type CommandSiteCreateFlags struct {
	EnableLinkAccess        bool
	LinkAccessType          string
	ServiceAccount          string
	Output                  string
	Host                    string
	Timeout                 time.Duration
	BindHost                string
	SubjectAlternativeNames []string
	Wait                    string
}

type CommandSiteUpdateFlags struct {
	EnableLinkAccess        bool
	LinkAccessType          string
	ServiceAccount          string
	Output                  string
	Host                    string
	Timeout                 time.Duration
	BindHost                string
	SubjectAlternativeNames []string
	Wait                    string
}

type CommandSiteDeleteFlags struct {
	All     bool
	Timeout time.Duration
	Wait    bool
}

type CommandLinkGenerateFlags struct {
	TlsCredentials     string
	Cost               string
	Output             string
	GenerateCredential bool
	Timeout            time.Duration
}
type CommandLinkUpdateFlags struct {
	TlsCredentials string
	Cost           string
	Output         string
	Timeout        time.Duration
	Wait           string
}

type CommandLinkDeleteFlags struct {
	Timeout time.Duration
	Wait    bool
}
type CommandLinkStatusFlags struct {
	Output string
}

type CommandTokenIssueFlags struct {
	Name               string
	Timeout            time.Duration
	ExpirationWindow   time.Duration
	RedemptionsAllowed int
	Cost               string
}

type CommandTokenRedeemFlags struct {
	Timeout time.Duration
}

type CommandConnectorCreateFlags struct {
	RoutingKey          string
	Host                string
	Selector            string
	TlsCredentials      string
	ConnectorType       string
	IncludeNotReadyPods bool
	Workload            string
	Timeout             time.Duration
	Output              string
	Wait                string
}

type CommandConnectorUpdateFlags struct {
	RoutingKey          string
	Host                string
	TlsCredentials      string
	ConnectorType       string
	Port                int
	Workload            string
	Selector            string
	IncludeNotReadyPods bool
	Timeout             time.Duration
	Output              string
	Wait                string
}

type CommandConnectorDeleteFlags struct {
	Timeout time.Duration
	Wait    bool
}

type CommandConnectorStatusFlags struct {
	Output string
}

type CommandListenerCreateFlags struct {
	RoutingKey     string
	Host           string
	TlsCredentials string
	ListenerType   string
	Timeout        time.Duration
	Output         string
	Wait           string
}

type CommandListenerUpdateFlags struct {
	RoutingKey     string
	Host           string
	TlsCredentials string
	ListenerType   string
	Timeout        time.Duration
	Port           int
	Output         string
	Wait           string
}

type CommandListenerStatusFlags struct {
	Output string
}

type CommandListenerDeleteFlags struct {
	Timeout time.Duration
	Wait    bool
}

type CommandSystemSetupFlags struct {
	Path     string
	Strategy string
	Force    bool
}

type CommandVersionFlags struct {
	Output string
}

type CommandDebugFlags struct {
}
