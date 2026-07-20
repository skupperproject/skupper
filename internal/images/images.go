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
	PrometheusServerImageName string = "prometheus:v3.11.3"
	NginxImageRegistry        string = "mirror.gcr.io/nginxinc"
	NginxImageName            string = "nginx-unprivileged:1.31.0-alpine"
	OauthProxyImageRegistry   string = "quay.io/openshift"
	OauthProxyImageName       string = "origin-oauth-proxy:4.14.0"
)

var (
	DefaultComponents    = []string{"router", "controller", "network-observer", "cli", "system-controller"}
	DevelopmentImageTags = []string{"main", "v2-dev"}
)
