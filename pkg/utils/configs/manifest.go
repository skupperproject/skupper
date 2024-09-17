package configs

import (
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/pkg/images"
	"os"
	"strings"
)

type SkupperImage struct {
	Name       string `json:"name"`
	SHA        string `json:"sha,omitempty"`
	Repository string `json:"repository,omitempty"`
}

type Manifest struct {
	Images    []SkupperImage     `json:"images"`
	Variables *map[string]string `json:"variables,omitempty"`
}

type ManifestManager struct {
	EnableSHA bool
}

type ManifestGenerator interface {
	GetConfiguredManifest() Manifest
	GetDefaultManifestWithEnv() Manifest
	CreateFile(m Manifest) error
}

func (manager *ManifestManager) GetConfiguredManifest() Manifest {
	return Manifest{
		Images: getSkupperConfiguredImages(manager.EnableSHA),
	}
}

func (manager *ManifestManager) GetDefaultManifestWithEnv() Manifest {
	return Manifest{
		Images:    getSkupperDefaultImages(),
		Variables: getEnvironmentVariableMap(),
	}
}

func (manager *ManifestManager) CreateFile(m Manifest) error {
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

func getSkupperConfiguredImages(enableSHA bool) []SkupperImage {
	return []SkupperImage{
		{
			Name:       images.GetRouterImageName(),
			SHA:        getSHAIfEnabled(enableSHA, images.GetRouterImageName()),
			Repository: "https://github.com/skupperproject/skupper-router",
		},
		{
			Name:       images.GetControllerImageName(),
			SHA:        getSHAIfEnabled(enableSHA, images.GetControllerImageName()),
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       images.GetConfigSyncImageName(),
			SHA:        getSHAIfEnabled(enableSHA, images.GetConfigSyncImageName()),
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       images.GetNetworkConsoleCollectorImageName(),
			SHA:        getSHAIfEnabled(enableSHA, images.GetNetworkConsoleCollectorImageName()),
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       images.GetBootstrapImageName(),
			SHA:        getSHAIfEnabled(enableSHA, images.GetBootstrapImageName()),
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name: images.GetPrometheusServerImageName(),
			SHA:  getSHAIfEnabled(enableSHA, images.GetPrometheusServerImageName()),
		},
		{
			Name: images.GetOauthProxyImageName(),
			SHA:  getSHAIfEnabled(enableSHA, images.GetOauthProxyImageName()),
		},
	}
}

func getSkupperDefaultImages() []SkupperImage {
	return []SkupperImage{
		{
			Name:       strings.Join([]string{images.DefaultImageRegistry, images.RouterImageName}, "/"),
			Repository: "https://github.com/skupperproject/skupper-router",
		},
		{
			Name:       strings.Join([]string{images.DefaultImageRegistry, images.ConfigSyncImageName}, "/"),
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name: strings.Join([]string{images.PrometheusImageRegistry, images.PrometheusServerImageName}, "/"),
		},
		{
			Name: strings.Join([]string{images.OauthProxyImageRegistry, images.OauthProxyImageName}, "/"),
		},
	}
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
