package images

const (
	DefaultImageRegistry             string = "quay.io/skupper"
	RouterImageName                  string = "skupper-router:main"
	ControllerImageName              string = "controller:v2-latest"
	AdaptorImageName                 string = "kube-adaptor:v2-latest"
	NetworkConsoleCollectorImageName string = "network-console-collector:v2-latest"
	BootstrapImageName               string = "bootstrap:v2-latest"

	PrometheusImageRegistry   string = "quay.io/prometheus"
	PrometheusServerImageName string = "prometheus:v2.42.0"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"

	// These constants will be soon deprecated.
	ServiceControllerImageName string = "service-controller:main"
	FlowCollectorImageName     string = "flow-collector:main"
	SiteControllerImageName    string = "site-controller:main"
)
