package configs

import (
	"os"

	"github.com/skupperproject/skupper/pkg/images"
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
	EnableSHA  bool
	Components []string
}

type ManifestGenerator interface {
	GetConfiguredManifest() SkupperManifest
	GetDefaultManifestWithEnv() Manifest
}

func (manager *ManifestManager) GetConfiguredManifest() SkupperManifest {
	return getSkupperConfiguredImages(manager.Components, manager.EnableSHA)
}

func (manager *ManifestManager) GetDefaultManifestWithEnv() Manifest {
	return Manifest{
		Images:    getSkupperDefaultImages(),
		Variables: getEnvironmentVariableMap(),
	}
}

func getSkupperConfiguredImages(components []string, enableSHA bool) SkupperManifest {
	var manifest SkupperManifest

	for _, component := range components {
		var image SkupperComponent
		image.Component = component
		image.Version = images.GetImageVersion(component)
		image.Images = images.GetImages(component, enableSHA)
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

	configSyncImage := os.Getenv(images.ConfigSyncImageEnvKey)
	if configSyncImage != "" {
		envVariables[images.ConfigSyncImageEnvKey] = configSyncImage
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
