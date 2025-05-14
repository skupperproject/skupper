package common

var (
	LinkAccessTypes = []string{"route", "loadbalancer", "default"}
	OutputTypes     = []string{"json", "yaml"}
	ListenerTypes   = []string{"tcp"}
	ConnectorTypes  = []string{"tcp"}
	WorkloadTypes   = []string{"deployment", "service", "daemonset", "statefulset"}
	WaitStatusTypes = []string{"ready", "configured", "none"}
	BundleTypes     = []string{"tarball", "shell-script"}
)

const (
	Connectors     string = "Connector"
	Listeners      string = "Listener"
	Sites          string = "Site"
	RouterAccesses string = "RouterAccess"
	Links          string = "Link"
	AccessTokens   string = "AccessToken"
	Secrets        string = "Secret"
)

const (
	SiteConfigNameKey string = "name"
)

const (
	ENV_PLATFORM = "SKUPPER_PLATFORM"
)

type Platform string

const (
	PlatformKubernetes Platform = "kubernetes"
	PlatformPodman     Platform = "podman"
	PlatformDocker     Platform = "docker"
	PlatformLinux      Platform = "linux"
)

func (p Platform) IsKubernetes() bool {
	return p == "" || p == PlatformKubernetes
}

func (p Platform) IsContainerEngine() bool {
	return p == PlatformDocker || p == PlatformPodman
}

// Assembly constants
const (
	EdgeRole        string = "edge"
	InterRouterRole string = "inter-router"
)
