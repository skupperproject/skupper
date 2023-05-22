package images

const (
	DefaultImageRegistry             string = "quay.io/skupper"
	RouterImageName                  string = "skupper-router:main"
	ServiceControllerImageName       string = "service-controller:master"
	ServiceControllerPodmanImageName string = "service-controller-podman:master"
	ConfigSyncImageName              string = "config-sync:master"
	FlowCollectorImageName           string = "flow-collector:master"
	PrometheusImageRegistry          string = "quay.io/prometheus"
	PrometheusServerImageName        string = "prometheus:v2.42.0"
)
