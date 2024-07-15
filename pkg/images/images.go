package images

const (
	DefaultImageRegistry       string = "quay.io/skupper"
	RouterImageName            string = "skupper-router:2.6.1"
	ServiceControllerImageName string = "service-controller:1.7.3"
	ControllerPodmanImageName  string = "controller-podman:1.7.3"
	ConfigSyncImageName        string = "config-sync:1.7.3"
	FlowCollectorImageName     string = "flow-collector:1.7.3"
	SiteControllerImageName    string = "site-controller:1.7.3"
	PrometheusImageRegistry    string = "quay.io/prometheus"
	PrometheusServerImageName  string = "prometheus:v2.42.0"
	OauthProxyImageRegistry    string = "quay.io/openshift"
	OauthProxyImageName        string = "origin-oauth-proxy:4.14.0"
)
