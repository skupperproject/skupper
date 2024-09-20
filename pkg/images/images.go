package images

const (
	DefaultImageRegistry             string = "quay.io/skupper"
	RouterImageName                  string = "skupper-router:3.0.0"
	ControllerImageName              string = "controller:2.0.0-preview-1"
	ConfigSyncImageName              string = "config-sync:2.0.0-preview-1"
	NetworkConsoleCollectorImageName string = "network-console-collector:2.0.0-preview-1"
	BootstrapImageName               string = "bootstrap:2.0.0-preview-1"

	PrometheusImageRegistry   string = "quay.io/prometheus"
	PrometheusServerImageName string = "prometheus:v2.42.0"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"

	// These constants will be soon deprecated.
	ServiceControllerImageName string = "service-controller:main"
	FlowCollectorImageName     string = "flow-collector:main"
	SiteControllerImageName    string = "site-controller:main"
)
