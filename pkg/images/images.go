package images

const (
	DefaultImageRegistry       string = "quay.io/nluaces"
	RouterImageName            string = "skupper-router:main"
	ServiceControllerImageName string = "service-controller:collector-lite"
	ControllerPodmanImageName  string = "controller-podman:main"
	ConfigSyncImageName        string = "config-sync:collector-lite"
	FlowCollectorImageName     string = "flow-collector:collector-lite"
	SiteControllerImageName    string = "site-controller:main"
	PrometheusImageRegistry    string = "quay.io/prometheus"
	PrometheusServerImageName  string = "prometheus:v2.42.0"
	OauthProxyImageRegistry    string = "quay.io/openshift"
	OauthProxyImageName        string = "origin-oauth-proxy:4.14.0"
)
