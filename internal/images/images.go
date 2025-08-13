package images

const (
	DefaultImageRegistry      string = "quay.io/skupper"
	RouterImageName           string = "skupper-router:3.4.0"
	ControllerImageName       string = "controller:2.1.1"
	KubeAdaptorImageName      string = "kube-adaptor:2.1.1"
	NetworkObserverImageName  string = "network-observer:2.1.1"
	CliImageName              string = "cli:2.1.1"
	SystemControllerImageName string = "system-controller:2.1.1"

	PrometheusImageRegistry   string = "quay.io/prometheus"
	PrometheusServerImageName string = "prometheus:v2.42.0"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"
)

var (
	KubeComponents       = []string{"router", "controller", "network-observer", "cli", "prometheus", "origin-oauth-proxy"}
	NonKubeComponents    = []string{"router", "network-observer", "cli", "system-controller", "prometheus", "origin-oauth-proxy"}
	DefaultComponents    = []string{"router", "prometheus", "origin-oauth-proxy"}
	DevelopmentImageTags = []string{"main", "v2-dev"}
)
