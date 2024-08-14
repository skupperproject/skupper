package common

import "time"

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
	FlagNameOutput         = "output"
	FlagDescOutput         = "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml"
	FlagNameServiceAccount = "service-account"
	FlagDescServiceAccount = "the Kubernetes service account under which to run the Skupper controller"

	FlagNameTlsSecret          = "tls-secret"
	FlagDescTlsSecret          = "the name of a Kubernetes secret containing the generated or externally-supplied TLS credentials."
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

	FlagNameRoutingKey      = "routing-key"
	FlagDescRoutingKey      = "The identifier used to route traffic from Connectors to connectors"
	FlagNameHost            = "host"
	FlagDescHost            = "The hostname or IP address of the local Connector"
	FlagNameConnectorType   = "type"
	FlagDescConnectorType   = "The Connector type. Choices: [tcp]."
	FlagNameIncludeNotReady = "include-not-ready"
	FlagDescIncludeNotRead  = "If true, include server pods that are not in the ready state."
	FlagNameSelector        = "selector"
	FlagDescSelector        = "A Kubernetes label selector for specifying target server pods."
	FlagNameWorkload        = "workload"
	FlagDescWorkload        = "A Kubernetes label selector for specifying target server pods."

	FlagNameConnectorPort = "port"
	FlagDescConnectorPort = "The port of the local connector"

	FlagNameConnectorStatusOutput = "output"
	FlagDescConnectorStatusOutput = "print status of connectors Choices: json, yaml"
)

type CommandSiteCreateFlags struct {
	EnableLinkAccess bool
	LinkAccessType   string
	ServiceAccount   string
	Output           string
}

type CommandSiteUpdateFlags struct {
	EnableLinkAccess bool
	LinkAccessType   string
	ServiceAccount   string
	Output           string
}

type CommandLinkGenerateFlags struct {
	TlsSecret          string
	Cost               string
	Output             string
	GenerateCredential bool
	Timeout            string
}
type CommandLinkUpdateFlags struct {
	TlsSecret string
	Cost      string
	Output    string
	Timeout   string
}

type CommandLinkDeleteFlags struct {
	Timeout string
}
type CommandLinkStatusFlags struct {
	Output string
}

type CommandTokenIssueFlags struct {
	Timeout            time.Duration
	ExpirationWindow   time.Duration
	RedemptionsAllowed int
}

type CommandTokenRedeemFlags struct {
	Timeout time.Duration
}

type CommandConnectorCreateFlags struct {
	RoutingKey      string
	Host            string
	Selector        string
	TlsSecret       string
	ConnectorType   string
	IncludeNotReady bool
	Workload        string
	Timeout         time.Duration
	Output          string
}

type CommandConnectorUpdateFlags struct {
	RoutingKey      string
	Host            string
	TlsSecret       string
	ConnectorType   string
	Port            int
	Workload        string
	Selector        string
	IncludeNotReady bool
	Timeout         time.Duration
	Output          string
}

type CommandConnectorDeleteFlags struct {
	Timeout time.Duration
}

type CommandConnectorStatusFlags struct {
	Output string
}
