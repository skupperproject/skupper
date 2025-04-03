package configs

import (
	"os"
	"os/exec"

	"github.com/skupperproject/skupper/internal/images"
)

type SkupperManifest struct {
	Components []SkupperComponent `json:"components"`
}

type SkupperComponent struct {
	Component string                `json:"component"`
	Version   string                `json:"version"`
	Images    []images.SkupperImage `json:"images"`
}

type Manifest struct {
	Images    SkupperManifest
	Variables *map[string]string `json:"variables,omitempty"`
}

type ManifestManager struct {
	EnableSHA   bool
	Components  []string
	RunningPods map[string]string
}

type ManifestGenerator interface {
	GetConfiguredManifest() SkupperManifest
	GetDefaultManifestWithEnv() Manifest
}

func (manager *ManifestManager) GetConfiguredManifest() SkupperManifest {
	return getSkupperImages(manager.Components, manager.EnableSHA, manager.RunningPods)
}

func (manager *ManifestManager) GetDefaultManifestWithEnv() Manifest {
	return Manifest{
		Images:    getSkupperDefaultImages(),
		Variables: getEnvironmentVariableMap(),
	}
}

func getSkupperImages(components []string, enableSHA bool, runningPods map[string]string) SkupperManifest {
	var manifest SkupperManifest

	// if Docker is not installed we can not inspect digests from images.
	_, err := exec.LookPath("docker")
	if err != nil {
		enableSHA = false
	}

	for _, component := range components {
		var image SkupperComponent
		image.Component = component
		image.Version = images.GetImageVersion(component)
		image.Images = images.GetImages(component, enableSHA)
		if runningPods[component] != "" {
			image.Version = images.GetVersionFromTag(runningPods[component])
			image.Images = GetRunningImages(component, enableSHA, runningPods)
		}

		manifest.Components = append(manifest.Components, image)
	}

	return manifest
}

func getSkupperDefaultImages() SkupperManifest {
	var manifest SkupperManifest

	for _, component := range images.DefaultComponents {
		var image SkupperComponent
		image.Component = component
		image.Version = images.GetImageVersion(component)
		image.Images = images.GetImages(component, false)
		manifest.Components = append(manifest.Components, image)
	}

	return manifest
}

func getEnvironmentVariableMap() *map[string]string {

	envVariables := make(map[string]string)

	skupperImageRegistry := os.Getenv(images.SkupperImageRegistryEnvKey)
	if skupperImageRegistry != "" {
		envVariables[images.SkupperImageRegistryEnvKey] = skupperImageRegistry
	}

	prometheusImageRegistry := os.Getenv(images.PrometheusImageRegistryEnvKey)
	if prometheusImageRegistry != "" {
		envVariables[images.PrometheusImageRegistry] = prometheusImageRegistry
	}

	oauthImageRegistry := os.Getenv(images.OauthProxyImageRegistry)
	if oauthImageRegistry != "" {
		envVariables[images.OauthProxyImageRegistry] = oauthImageRegistry
	}

	routerImage := os.Getenv(images.RouterImageEnvKey)
	if routerImage != "" {
		envVariables[images.RouterImageEnvKey] = routerImage
	}

	controllerImage := os.Getenv(images.ControllerImageEnvKey)
	if controllerImage != "" {
		envVariables[images.ControllerImageEnvKey] = controllerImage
	}

	adaptorImage := os.Getenv(images.KubeAdaptorImageEnvKey)
	if adaptorImage != "" {
		envVariables[images.KubeAdaptorImageEnvKey] = adaptorImage
	}

	networkObserverImage := os.Getenv(images.NetworkObserverImageEnvKey)
	if networkObserverImage != "" {
		envVariables[images.NetworkObserverImageEnvKey] = networkObserverImage
	}

	cliImage := os.Getenv(images.CliImageEnvKey)
	if cliImage != "" {
		envVariables[images.CliImageEnvKey] = cliImage
	}

	prometheusImage := os.Getenv(images.PrometheusServerImageEnvKey)
	if prometheusImage != "" {
		envVariables[images.PrometheusServerImageEnvKey] = prometheusImage
	}

	oauthImage := os.Getenv(images.OauthProxyImageEnvKey)
	if oauthImage != "" {
		envVariables[images.OauthProxyImageEnvKey] = oauthImage
	}

	return &envVariables
}

func GetRunningImages(component string, enableSHA bool, runningPods map[string]string) []images.SkupperImage {

	names := make(map[string]string)

	switch component {
	case "router":
		// skupper router has two components
		names[images.RouterImageEnvKey] = runningPods["router"]
		names[images.KubeAdaptorImageEnvKey] = runningPods["kube-adaptor"]

	case "controller":
		names[images.ControllerImageEnvKey] = runningPods["controller"]

	case "network-observer":

		names[images.NetworkObserverImageEnvKey] = runningPods["network-observer"]
	}

	if names != nil {
		return images.GetImage(names, "", enableSHA)
	}

	return nil
}
