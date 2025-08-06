package images

const (
	DefaultImageRegistry       string = "quay.io/skupper"
	RouterImageName            string = "skupper-router:2.7.6"
	ServiceControllerImageName string = "service-controller:1.9.4"
	ControllerPodmanImageName  string = "controller-podman:1.9.4"
	ConfigSyncImageName        string = "config-sync:1.9.4"
	FlowCollectorImageName     string = "flow-collector:1.9.4"
	SiteControllerImageName    string = "site-controller:1.9.4"
	PrometheusImageRegistry    string = "quay.io/prometheus"
	PrometheusServerImageName  string = "prometheus:v2.55.1"
	OauthProxyImageRegistry    string = "quay.io/openshift"
	OauthProxyImageName        string = "origin-oauth-proxy:4.18.0"
)
