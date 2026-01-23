package images

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/strings/slices"
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
	SystemControllerImageEnvKey   string = "SKUPPER_SYSTEM_CONTROLLER_IMAGE"
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

func GetSystemControllerImageName() string {
	image := os.Getenv(SystemControllerImageEnvKey)
	if image == "" {
		imageRegistry := GetImageRegistry()
		return strings.Join([]string{imageRegistry, SystemControllerImageName}, "/")
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

func CreateMapImageDigest(runningPods map[string]string) map[string]string {
	imagesToRetrieve := map[string]string{
		"router":             GetRouterImageName(),
		"controller":         GetControllerImageName(),
		"kube-adaptor":       GetKubeAdaptorImageName(),
		"network-observer":   GetNetworkObserverImageName(),
		"cli":                GetCliImageName(),
		"system-controller":  GetSystemControllerImageName(),
		"prometheus":         GetPrometheusServerImageName(),
		"origin-oauth-proxy": GetOauthProxyImageName(),
	}

	//update the list of images to retrieve with the running containers
	for podName, podImage := range runningPods {
		imagesToRetrieve[podName] = podImage
	}

	completedImages := make(map[string]string)
	var waitGroup sync.WaitGroup
	imageChannel := make(chan SkupperImage, len(imagesToRetrieve))

	for _, image := range imagesToRetrieve {

		waitGroup.Add(1)

		go func(skImage string) {

			tag := strings.Split(skImage, ":")[1]
			isDevImageTag := slices.Contains(DevelopmentImageTags, tag)

			defer waitGroup.Done()

			var output bytes.Buffer
			checkCmd := exec.Command("docker", "inspect", "--format={{index .RepoDigests 0}}", skImage)
			checkCmd.Stdout = &output
			checkCmd.Stderr = &output

			var pullErr error
			var inspectErr error

			if err := checkCmd.Run(); err != nil || isDevImageTag {
				// Pull the image only if it's not present locally or if it's an image that could be overwritten.
				pullCmd := exec.Command("docker", "pull", skImage)
				if pullErr = pullCmd.Run(); pullErr != nil {
					slog.Error("Error pulling image", slog.String("error", output.String()))
				}

				// Retry inspection after pulling
				output.Reset()
				checkCmd = exec.Command("docker", "inspect", "--format={{index .RepoDigests 0}}", skImage)
				checkCmd.Stdout = &output
				checkCmd.Stderr = &output
				if inspectErr = checkCmd.Run(); inspectErr != nil {
					slog.Error("Error getting image digest", slog.String("error", output.String()))
				}
			}

			if pullErr == nil && inspectErr == nil {
				digestBytes := output.String()
				imageWithoutTag := strings.Split(skImage, ":")[0]

				// Extract and print the digest
				parsedDigest := strings.ReplaceAll(strings.ReplaceAll(digestBytes, "'", ""), "\n", "")
				digest := strings.TrimPrefix(strings.Trim(parsedDigest, "'"), imageWithoutTag+"@")

				imageChannel <- SkupperImage{Name: skImage, Digest: digest}
			}
		}(image)
	}

	go func() {
		waitGroup.Wait()
		close(imageChannel)
	}()

	for img := range imageChannel {
		completedImages[img.Name] = img.Digest
	}

	return completedImages
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

func GetImage(imageNames map[string]string, imageRegistry string, digestMap map[string]string) []SkupperImage {
	var skupperImage []SkupperImage

	for _, name := range imageNames {

		var image SkupperImage

		image.Name = name

		if digestMap != nil {
			image.Digest = digestMap[name]
		}

		skupperImage = append(skupperImage, image)
	}

	return skupperImage

}

func GetImages(component string, digestMap map[string]string) []SkupperImage {
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
			names[RouterImageEnvKey] = strings.Join([]string{GetImageRegistry(), RouterImageName}, "/")
		}

		envImage = os.Getenv(KubeAdaptorImageEnvKey)
		if envImage != "" {
			names[KubeAdaptorImageEnvKey] = envImage
		} else {
			names[KubeAdaptorImageEnvKey] = strings.Join([]string{GetImageRegistry(), KubeAdaptorImageName}, "/")
		}
	case "controller":
		envImage := os.Getenv(ControllerImageEnvKey)

		if envImage != "" {
			names[ControllerImageEnvKey] = envImage
		} else {
			names[ControllerImageEnvKey] = strings.Join([]string{GetImageRegistry(), ControllerImageName}, "/")
		}

	case "network-observer":

		envImage := os.Getenv(NetworkObserverImageEnvKey)

		if envImage != "" {
			names[NetworkObserverImageEnvKey] = envImage
		} else {
			names[NetworkObserverImageEnvKey] = strings.Join([]string{GetImageRegistry(), NetworkObserverImageName}, "/")
		}

	case "cli":
		names[CliImageEnvKey] = strings.Join([]string{GetImageRegistry(), CliImageName}, "/")
	case "system-controller":
		names[SystemControllerImageEnvKey] = strings.Join([]string{GetImageRegistry(), SystemControllerImageName}, "/")
	case "prometheus":
		names[PrometheusServerImageEnvKey] = strings.Join([]string{GetPrometheusImageRegistry(), PrometheusServerImageName}, "/")
	case "origin-oauth-proxy":
		names[OauthProxyImageEnvKey] = strings.Join([]string{GetOauthProxyImageRegistry(), OauthProxyImageName}, "/")
	}

	if names != nil {
		return GetImage(names, registry, digestMap)
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
		image = os.Getenv(CliImageEnvKey)
		if image == "" {
			image = ControllerImageName
		}
	case "system-controller":
		image = os.Getenv(SystemControllerImageEnvKey)
		if image == "" {
			image = SystemControllerImageName
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
