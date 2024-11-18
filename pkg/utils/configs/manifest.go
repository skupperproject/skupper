package configs

import (
	"encoding/json"
	"fmt"
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
	CreateFile(m SkupperManifest) error
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

func (manager *ManifestManager) CreateFile(m SkupperManifest) error {
	filename := "manifest.json"

	// Encode the manifest image list as JSON.
	manifestListJSON, err := json.MarshalIndent(m, "", "   ")
	if err != nil {
		return fmt.Errorf("Error encoding manifest image list: %v\n", err)

	}

	// Create a new file.
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Error creating file: %v\n", err)
	}

	// Write the JSON data to the file.
	_, err = file.Write(manifestListJSON)
	if err != nil {
		return fmt.Errorf("Error writing to file: %v\n", err)
	}

	fmt.Printf("%s file successfully generated in the current directory.\n", filename)
	return nil
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

	flowNetworkConsoleCollectorImage := os.Getenv(images.NetworkConsoleCollectorImageEnvKey)
	if flowNetworkConsoleCollectorImage != "" {
		envVariables[images.NetworkConsoleCollectorImageEnvKey] = flowNetworkConsoleCollectorImage
	}

	bootstrapImage := os.Getenv(images.BootstrapImageEnvKey)
	if bootstrapImage != "" {
		envVariables[images.BootstrapImageEnvKey] = bootstrapImage
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

func getSHAIfEnabled(enableSHA bool, imageName string) string {
	if !enableSHA {
		return ""
	}
	return images.GetSha(imageName)
}
