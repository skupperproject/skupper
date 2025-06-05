package images

const (
	DefaultImageRegistry       string = "quay.io/skupper"
	RouterImageName            string = "skupper-router:2.4.3"
	ServiceControllerImageName string = "service-controller:1.4.8"
	ConfigSyncImageName        string = "config-sync:1.4.8"
	FlowCollectorImageName     string = "flow-collector:1.4.8"
	SiteControllerImageName    string = "site-controller:1.4.8"
	PrometheusImageRegistry    string = "quay.io/prometheus"
	PrometheusServerImageName  string = "prometheus:v2.42.0"
	OauthProxyImageRegistry    string = "quay.io/openshift"
	OauthProxyImageName        string = "origin-oauth-proxy:4.14.0"
)
