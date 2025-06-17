package images

const (
	DefaultImageRegistry       string = "quay.io/skupper"
	RouterImageName            string = "skupper-router:2.7.5"
	ServiceControllerImageName string = "service-controller:1.8.5"
	ControllerPodmanImageName  string = "controller-podman:1.8.5"
	ConfigSyncImageName        string = "config-sync:1.8.5"
	FlowCollectorImageName     string = "flow-collector:1.8.5"
	SiteControllerImageName    string = "site-controller:1.8.5"
	PrometheusImageRegistry    string = "quay.io/prometheus"
	PrometheusServerImageName  string = "prometheus:v2.55.1"
	OauthProxyImageRegistry    string = "quay.io/openshift"
	OauthProxyImageName        string = "origin-oauth-proxy:4.18.0"
)
