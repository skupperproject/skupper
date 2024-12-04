package images

const (
	DefaultImageRegistry     string = "quay.io/skupper"
	RouterImageName          string = "skupper-router:main"
	ControllerImageName      string = "controller:v2-latest"
	ConfigSyncImageName      string = "config-sync:v2-latest"
	NetworkObserverImageName string = "network-observer:v2-latest"
	BootstrapImageName       string = "bootstrap:v2-latest"

	PrometheusImageRegistry   string = "quay.io/prometheus"
	PrometheusServerImageName string = "prometheus:v2.42.0"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"
)

var (
	KubeComponents    = []string{"router", "controller", "network-observer", "bootstrap", "prometheus", "origin-oauth-proxy"}
	NonKubeComponents = []string{"router", "network-observer", "bootstrap", "prometheus", "origin-oauth-proxy"}
	DefaultComponents = []string{"router", "prometheus", "origin-oauth-proxy"}
)
