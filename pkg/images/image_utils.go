package images

import (
	"os"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
)

const (
	RouterImageEnvKey                       string = "QDROUTERD_IMAGE"
	ServiceControllerImageEnvKey            string = "SKUPPER_SERVICE_CONTROLLER_IMAGE"
	ServiceControllerPodmanImageEnvKey      string = "SKUPPER_SERVICE_CONTROLLER_PODMAN_IMAGE"
	ConfigSyncImageEnvKey                   string = "SKUPPER_CONFIG_SYNC_IMAGE"
	FlowCollectorImageEnvKey                string = "SKUPPER_FLOW_COLLECTOR_IMAGE"
	PrometheusServerImageEnvKey             string = "PROMETHEUS_SERVER_IMAGE"
	RouterPullPolicyEnvKey                  string = "QDROUTERD_IMAGE_PULL_POLICY"
	ServiceControllerPullPolicyEnvKey       string = "SKUPPER_SERVICE_CONTROLLER_IMAGE_PULL_POLICY"
	ServiceControllerPodmanPullPolicyEnvKey string = "SKUPPER_SERVICE_CONTROLLER_PODMAN_IMAGE_PULL_POLICY"
	ConfigSyncPullPolicyEnvKey              string = "SKUPPER_CONFIG_SYNC_IMAGE_PULL_POLICY"
	FlowCollectorPullPolicyEnvKey           string = "SKUPPER_FLOW_COLLECTOR_IMAGE_PULL_POLICY"
	PrometheusServerPullPolicyEnvKey        string = "PROMETHEUS_SERVER_IMAGE_PULL_POLICY"
	SkupperImageRegistryEnvKey              string = "SKUPPER_IMAGE_REGISTRY"
	PrometheusImageRegistryEnvKey           string = "PROMETHEUS_IMAGE_REGISTRY"
)

func getPullPolicy(key string) string {
	policy := os.Getenv(key)
	if policy == "" {
		policy = string(corev1.PullAlways)
	}
	return policy
}

func GetRouterImageName() string {
	image := os.Getenv(RouterImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, RouterImageName}, "/")

	} else {
		return image
	}
}

func GetRouterImagePullPolicy() string {
	return getPullPolicy(RouterPullPolicyEnvKey)
}

func GetRouterImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetRouterImageName(),
		PullPolicy: GetRouterImagePullPolicy(),
	}
}

func AddRouterImageOverrideToEnv(env []corev1.EnvVar) []corev1.EnvVar {
	result := env
	image := os.Getenv(RouterImageEnvKey)
	if image != "" {
		result = append(result, corev1.EnvVar{Name: RouterImageEnvKey, Value: image})
	}
	policy := os.Getenv(RouterPullPolicyEnvKey)
	if policy != "" {
		result = append(result, corev1.EnvVar{Name: RouterPullPolicyEnvKey, Value: policy})
	}
	return result
}

func GetServiceControllerImageName() string {
	image := os.Getenv(ServiceControllerImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, ServiceControllerImageName}, "/")
	} else {
		return image
	}
}

func GetServiceControllerImagePullPolicy() string {
	return getPullPolicy(ServiceControllerPullPolicyEnvKey)
}

func GetServiceControllerImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetServiceControllerImageName(),
		PullPolicy: GetServiceControllerImagePullPolicy(),
	}
}

func GetServiceControllerPodmanImageName() string {
	image := os.Getenv(ServiceControllerPodmanImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, ServiceControllerPodmanImageName}, "/")
	} else {
		return image
	}
}

func GetServiceControllerPodmanImagePullPolicy() string {
	return getPullPolicy(ServiceControllerPodmanPullPolicyEnvKey)
}

func GetServiceControllerPodmanImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetServiceControllerPodmanImageName(),
		PullPolicy: GetServiceControllerPodmanImagePullPolicy(),
	}
}

func GetConfigSyncImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetConfigSyncImageName(),
		PullPolicy: GetConfigSyncImagePullPolicy(),
	}
}

func GetConfigSyncImageName() string {
	image := os.Getenv(ConfigSyncImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, ConfigSyncImageName}, "/")
	} else {
		return image
	}
}

func GetConfigSyncImagePullPolicy() string {
	return getPullPolicy(ConfigSyncPullPolicyEnvKey)
}

func GetFlowCollectorImageName() string {
	image := os.Getenv(FlowCollectorImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, FlowCollectorImageName}, "/")
	} else {
		return image
	}
}

func GetFlowCollectorImagePullPolicy() string {
	return getPullPolicy(FlowCollectorPullPolicyEnvKey)
}

func GetFlowCollectorImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetFlowCollectorImageName(),
		PullPolicy: GetFlowCollectorImagePullPolicy(),
	}
}

func GetPrometheusServerImageName() string {
	image := os.Getenv(PrometheusServerImageEnvKey)
	if image == "" {
		imageRegistry := GetPrometheusImageRegistry()
		return strings.Join([]string{imageRegistry, PrometheusServerImageName}, "/")
	} else {
		return image
	}
}

func GetPrometheusServerImagePullPolicy() string {
	return getPullPolicy(PrometheusServerPullPolicyEnvKey)
}

func GetPrometheusServerImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetPrometheusServerImageName(),
		PullPolicy: GetPrometheusServerImagePullPolicy(),
	}
}

func GetImageRegistry() string {
	imageRegistry := os.Getenv(SkupperImageRegistryEnvKey)
	if imageRegistry == "" {
		return DefaultImageRegistry
	}
	return imageRegistry
}

func GetPrometheusImageRegistry() string {
	imageRegistry := os.Getenv(PrometheusImageRegistryEnvKey)
	if imageRegistry == "" {
		return PrometheusImageRegistry
	}
	return imageRegistry
}
