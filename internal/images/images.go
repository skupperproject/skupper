package images

const (
	DefaultImageRegistry      string = "quay.io/skupper"
	RouterImageName           string = "skupper-router:3.5.1"
	ControllerImageName       string = "controller:2.2.1"
	KubeAdaptorImageName      string = "kube-adaptor:2.2.1"
	NetworkObserverImageName  string = "network-observer:2.2.1"
	CliImageName              string = "cli:2.2.1"
	SystemControllerImageName string = "system-controller:2.2.1"

	PrometheusImageRegistry   string = "quay.io/prometheus"
	PrometheusServerImageName string = "prometheus:v2.42.0"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"
)

var (
	DefaultComponents    = []string{"router", "controller", "network-observer", "cli", "system-controller"}
	DevelopmentImageTags = []string{"main", "v2-dev"}
)
