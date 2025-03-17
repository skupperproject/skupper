package images

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type SkupperImage struct {
	Name   string `json:"name,omitempty"`
	Digest string `json:"digest,omitempty"`
}

const (
	RouterImageEnvKey             string = "SKUPPER_ROUTER_IMAGE"
	ControllerImageEnvKey         string = "SKUPPER_CONTROLLER_IMAGE"
	KubeAdaptorImageEnvKey        string = "SKUPPER_KUBE_ADAPTOR_IMAGE"
	NetworkObserverImageEnvKey    string = "SKUPPER_NETWORK_OBSERVER_IMAGE"
	CliImageEnvKey                string = "SKUPPER_CLI_IMAGE"
	PrometheusServerImageEnvKey   string = "PROMETHEUS_SERVER_IMAGE"
	OauthProxyImageEnvKey         string = "OAUTH_PROXY_IMAGE"
	RouterPullPolicyEnvKey        string = "SKUPPER_ROUTER_IMAGE_PULL_POLICY"
	KubeAdaptorPullPolicyEnvKey   string = "SKUPPER_KUBE_ADAPTOR_IMAGE_PULL_POLICY"
	OauthProxyPullPolicyEnvKey    string = "OAUTH_PROXY_IMAGE_PULL_POLICY"
	SkupperImageRegistryEnvKey    string = "SKUPPER_IMAGE_REGISTRY"
	PrometheusImageRegistryEnvKey string = "PROMETHEUS_IMAGE_REGISTRY"
	OauthProxyRegistryEnvKey      string = "OAUTH_PROXY_IMAGE_REGISTRY"
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

func GetControllerImageName() string {
	image := os.Getenv(ControllerImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, ControllerImageName}, "/")
	} else {
		return image
	}
}

func GetNetworkObserverImageName() string {
	image := os.Getenv(NetworkObserverImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, NetworkObserverImageName}, "/")
	} else {
		return image
	}
}

func GetCliImageName() string {
	image := os.Getenv(CliImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, CliImageName}, "/")
	} else {
		return image
	}
}

func GetKubeAdaptorImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetKubeAdaptorImageName(),
		PullPolicy: GetKubeAdaptorImagePullPolicy(),
	}
}

func GetKubeAdaptorImageName() string {
	image := os.Getenv(KubeAdaptorImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, KubeAdaptorImageName}, "/")
	} else {
		return image
	}
}

func GetKubeAdaptorImagePullPolicy() string {
	return getPullPolicy(KubeAdaptorPullPolicyEnvKey)
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

func GetSha(imageName string) string {
	checkCmd := exec.Command("docker", "inspect", imageName)
	if err := checkCmd.Run(); err != nil {
		// Pull the image only if it's not present locally
		pullCmd := exec.Command("docker", "pull", imageName)
		if err := pullCmd.Run(); err != nil {
			fmt.Printf("Error pulling image: %v\n", err)
			return err.Error()
		}
	}

	// Get the image digest
	digestCmd := exec.Command("docker", "inspect", "--format='{{index .RepoDigests 0}}'", imageName)
	digestBytes, err := digestCmd.Output()
	if err != nil {
		fmt.Printf("Error getting image digest: %v", err)
		return err.Error()
	}

	imageWithoutTag := strings.Split(imageName, ":")[0]

	// Extract and print the digest
	parsedDigest := strings.ReplaceAll(strings.ReplaceAll(string(digestBytes), "'", ""), "\n", "")
	digest := strings.TrimPrefix(strings.Trim(parsedDigest, "'"), imageWithoutTag+"@")

	return digest
}

func GetOauthProxyImageName() string {
	image := os.Getenv(OauthProxyImageEnvKey)
	if image == "" {
		imageRegistry := GetOauthProxyImageRegistry()
		return strings.Join([]string{imageRegistry, OauthProxyImageName}, "/")
	} else {
		return image
	}
}

func GetOauthProxyImagePullPolicy() string {
	return getPullPolicy(OauthProxyPullPolicyEnvKey)
}

func GetOauthProxyImageDetails() types.ImageDetails {
	return types.ImageDetails{
		Name:       GetOauthProxyImageName(),
		PullPolicy: GetOauthProxyImagePullPolicy(),
	}
}

func GetOauthProxyImageRegistry() string {
	imageRegistry := os.Getenv(OauthProxyRegistryEnvKey)
	if imageRegistry == "" {
		return OauthProxyImageRegistry
	}
	return imageRegistry
}

func GetImage(imageNames map[string]string, imageRegistry string, enableSHA bool) []SkupperImage {
	var skupperImage []SkupperImage
	var waitGroup sync.WaitGroup
	imageChannel := make(chan SkupperImage, len(imageNames))

	for _, name := range imageNames {
		waitGroup.Add(1)

		go func(imageName string) {
			defer waitGroup.Done()
			var image SkupperImage

			if imageRegistry != "" {
				imageName = strings.Join([]string{imageRegistry, imageName}, "/")
			}
			image.Name = imageName

			if enableSHA {
				image.Digest = GetSha(imageName)
			}

			imageChannel <- image
		}(name)
	}

	go func() {
		waitGroup.Wait()
		close(imageChannel)
	}()

	for img := range imageChannel {
		skupperImage = append(skupperImage, img)
	}

	return skupperImage

}

func GetImages(component string, enableSHA bool) []SkupperImage {
	//var names map[string]string
	var registry string

	names := make(map[string]string)
	switch component {
	case "router":
		// skupper router has two components
		envImage := os.Getenv(RouterImageEnvKey)

		if envImage != "" {
			names[RouterImageEnvKey] = envImage
		} else {
			names[RouterImageEnvKey] = RouterImageName
			registry = GetImageRegistry()
		}

		envImage = os.Getenv(KubeAdaptorImageEnvKey)
		if envImage != "" {
			names[KubeAdaptorImageEnvKey] = envImage
		} else {
			names[KubeAdaptorImageEnvKey] = KubeAdaptorImageName
			registry = GetImageRegistry()
		}
	case "controller":
		envImage := os.Getenv(ControllerImageEnvKey)

		if envImage != "" {
			names[ControllerImageEnvKey] = envImage
		} else {
			names[ControllerImageEnvKey] = ControllerImageName
			registry = GetImageRegistry()
		}

	case "network-observer":

		envImage := os.Getenv(NetworkObserverImageEnvKey)

		if envImage != "" {
			names[NetworkObserverImageEnvKey] = envImage
		} else {
			names[NetworkObserverImageEnvKey] = NetworkObserverImageName
			registry = GetImageRegistry()
		}

	case "cli":
		names[CliImageEnvKey] = CliImageName
		registry = GetImageRegistry()
	case "prometheus":
		names[PrometheusServerImageEnvKey] = PrometheusServerImageName
		registry = GetPrometheusImageRegistry()
	case "origin-oauth-proxy":
		names[OauthProxyImageEnvKey] = OauthProxyImageName
		registry = GetOauthProxyImageRegistry()
	}

	if names != nil {
		return GetImage(names, registry, enableSHA)
	} else {
		return nil
	}
}

func GetImageVersion(component string) string {
	var image string

	switch component {
	case "router":
		envImage := os.Getenv(RouterImageEnvKey)

		if envImage != "" {
			image = envImage
		} else {
			image = RouterImageName
		}

	case "controller":

		envImage := os.Getenv(ControllerImageEnvKey)

		if envImage != "" {
			image = envImage
		} else {
			image = ControllerImageName
		}

	case "network-observer":

		envImage := os.Getenv(NetworkObserverImageEnvKey)

		if envImage != "" {
			image = envImage
		} else {
			image = NetworkObserverImageName
		}
	case "cli":
		image = os.Getenv(CliImageName)
		if image == "" {
			image = ControllerImageName
		}
	case "prometheus":
		image = os.Getenv(PrometheusServerImageEnvKey)
		if image == "" {
			image = PrometheusServerImageName
		}
	case "origin-oauth-proxy":
		image = os.Getenv(OauthProxyImageEnvKey)
		if image == "" {
			image = OauthProxyImageName
		}
	}
	if image != "" {
		return GetVersionFromTag(image)
	}
	return ""
}

func GetVersionFromTag(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) == 2 {
		return parts[1]
	}

	return ""
}
