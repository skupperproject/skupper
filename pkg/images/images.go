package images

const (
	DefaultImageRegistry       string = "quay.io/skupper"
	RouterImageName            string = "skupper-router:2.7.3"
	ServiceControllerImageName string = "service-controller:v1-dev"
	ControllerPodmanImageName  string = "controller-podman:v1-dev"
	ConfigSyncImageName        string = "config-sync:v1-dev"
	FlowCollectorImageName     string = "flow-collector:v1-dev"
	SiteControllerImageName    string = "site-controller:v1-dev"
	PrometheusImageRegistry    string = "quay.io/prometheus"
	PrometheusServerImageName  string = "prometheus:v2.55.1"
	OauthProxyImageRegistry    string = "quay.io/openshift"
	OauthProxyImageName        string = "origin-oauth-proxy:4.18.0"
)
