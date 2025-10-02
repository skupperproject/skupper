package common

import (
	"time"
)

const (
	FlagNamePlatform   = "platform"
	FlagDescPlatform   = "Set the platform type to use [kubernetes, podman, docker, linux]"
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
	FlagNameOutput    = "output"
	FlagDescOutput    = "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml"
	FlagVerboseOutput = "print verbose output to the console. Choices: json, yaml"

	FlagNameTlsCredentials     = "tls-credentials"
	FlagDescTlsCredentials     = "the name of a Kubernetes secret containing the generated or externally-supplied TLS credentials."
	FlagNameCost               = "cost"
	FlagDescCost               = "the configured \"expense\" of sending traffic over the link."
	FlagNameGenerateCredential = "generate-credential"
	FlagDescGenerateCredential = "generate the necessary credentials to create the link"
	FlagNameTimeout            = "timeout"
	FlagDescTimeout            = "raise an error if the operation does not complete in the given period of time (expressed in seconds)."
	FlagNameLinkName           = "name"
	FlagDescNameLinkName       = "Router Access Name"
	FlagNameLinkHost           = "host"
	FlagDescNameLinkHost       = "Endpoint Host"

	FlagNameRedemptionsAllowed = "redemptions-allowed"
	FlagDescRedemptionsAllowed = "The number of times an access token for this grant can be redeemed."
	FlagNameExpirationWindow   = "expiration-window"
	FlagDescExpirationWindow   = "The period of time in which an access token for this grant can be redeemed."

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
	FlagDescWorkload            = "A Kubernetes resource name that identifies a workload expressed like resource-type/resource-name. Expected resource types: service, daemonset, deployment, and statefulset."

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

	FlagNameForce = "force"

	FlagNameWait       = "wait"
	FlagDescWait       = "Wait for the given status before exiting. Choices: configured, ready, none"
	FlagDescDeleteWait = "Wait for deletion to complete before exiting"

	FlagNameAll       = "all"
	FlagDescDeleteAll = "delete all skupper resources associated with site in current namespace"

	FlagNameInput = "input"
	FlagDescInput = "The location of the Skupper resources defining the site."
	FlagNameType  = "type"
	FlagDescType  = "The bundle type to be produced. Choices: tarball, shell-script"

	FlagDescUninstallForce = "all existing sites (active or not) will be deleted"

	FlagNameHA = "enable-ha"
	FlagDescHA = "Configure the site for high availability (EnableHA). EnableHA sites have two active routers"

	FlagNameFileName = "filename"
	FlagDescFileName = "The name of the file with custom resources"
)

type CommandSiteCreateFlags struct {
	EnableLinkAccess bool
	LinkAccessType   string
	EnableHA         bool
	Timeout          time.Duration
	Wait             string
}

type CommandSiteUpdateFlags struct {
	EnableLinkAccess bool
	LinkAccessType   string
	EnableHA         bool
	Timeout          time.Duration
	Wait             string
}

type CommandSiteDeleteFlags struct {
	All     bool
	Timeout time.Duration
	Wait    bool
}

type CommandSiteStatusFlags struct {
	Output string
}

type CommandSiteGenerateFlags struct {
	EnableLinkAccess bool
	LinkAccessType   string
	EnableHA         bool
	Output           string
}

type CommandLinkGenerateFlags struct {
	TlsCredentials     string
	Cost               string
	Output             string
	GenerateCredential bool
	Timeout            time.Duration
	Name               string
	Host               string
}
type CommandLinkUpdateFlags struct {
	TlsCredentials string
	Cost           string
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
	Wait                string
}

type CommandConnectorDeleteFlags struct {
	Timeout time.Duration
	Wait    bool
}

type CommandConnectorStatusFlags struct {
	Output string
}

type CommandConnectorGenerateFlags struct {
	RoutingKey          string
	Host                string
	Selector            string
	TlsCredentials      string
	ConnectorType       string
	IncludeNotReadyPods bool
	Workload            string
	Output              string
}

type CommandListenerCreateFlags struct {
	RoutingKey     string
	Host           string
	TlsCredentials string
	ListenerType   string
	Timeout        time.Duration
	Wait           string
}

type CommandListenerUpdateFlags struct {
	RoutingKey     string
	Host           string
	TlsCredentials string
	ListenerType   string
	Timeout        time.Duration
	Port           int
	Wait           string
}

type CommandListenerStatusFlags struct {
	Output string
}

type CommandListenerDeleteFlags struct {
	Timeout time.Duration
	Wait    bool
}

type CommandListenerGenerateFlags struct {
	RoutingKey     string
	Host           string
	TlsCredentials string
	ListenerType   string
	Output         string
}

type CommandVersionFlags struct {
	Output string
}

type CommandDebugFlags struct {
}

type CommandSystemUninstallFlags struct {
	Force bool
}

type CommandSystemGenerateBundleFlags struct {
	Input string
	Type  string
}

type CommandSystemApplyFlags struct {
	Filename string
}

type CommandSystemDeleteFlags struct {
	Filename string
}
