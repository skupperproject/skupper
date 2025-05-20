package images

const (
	DefaultImageRegistry      string = "quay.io/skupper"
	RouterImageName           string = "skupper-router:main"
	ControllerImageName       string = "controller:v2-dev"
	KubeAdaptorImageName      string = "kube-adaptor:v2-dev"
	NetworkObserverImageName  string = "network-observer:v2-dev"
	CliImageName              string = "cli:v2-dev"
	SystemControllerImageName string = "system-controller:v2-dev"

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
