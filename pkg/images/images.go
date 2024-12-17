package images

const (
	DefaultImageRegistry     string = "quay.io/skupper"
	RouterImageName          string = "skupper-router:main"
	ControllerImageName      string = "controller:v2-latest"
	AdaptorImageName         string = "kube-adaptor:v2-latest"
	NetworkObserverImageName string = "network-observer:v2-latest"
	CliImageName             string = "cli:v2-latest"

	PrometheusImageRegistry   string = "quay.io/prometheus"
	PrometheusServerImageName string = "prometheus:v2.42.0"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"
)

var (
	KubeComponents    = []string{"router", "controller", "network-observer", "cli", "prometheus", "origin-oauth-proxy"}
	NonKubeComponents = []string{"router", "network-observer", "cli", "prometheus", "origin-oauth-proxy"}
	DefaultComponents = []string{"router", "prometheus", "origin-oauth-proxy"}
)
