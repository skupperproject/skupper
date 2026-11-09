package images

const (
	DefaultImageRegistry      string = "quay.io/skupper"
	RouterImageName           string = "skupper-router:3.5.4"
	ControllerImageName       string = "controller:2.3.0"
	KubeAdaptorImageName      string = "kube-adaptor:2.3.0"
	NetworkObserverImageName  string = "network-observer:2.3.0"
	CliImageName              string = "cli:2.3.0"
	SystemControllerImageName string = "system-controller:2.3.0"

	PrometheusImageRegistry   string = "quay.io/prometheus"
	PrometheusServerImageName string = "prometheus:v3.11.3"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"
)

var (
	DefaultComponents    = []string{"router", "controller", "network-observer", "cli", "system-controller"}
	DevelopmentImageTags = []string{"main", "v2-dev"}
)
