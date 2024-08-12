package common

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
