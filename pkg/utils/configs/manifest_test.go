package configs

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/images"
	"gotest.tools/assert"
	"os"
	"strings"
	"testing"
)

func TestManifestManager(t *testing.T) {
	testcases := []struct {
		title                          string
		envVariablesWithValue          []string
		expectedConfiguredManifest     Manifest
		expectedDefaultManifestWithEnv Manifest
	}{
		{
			title: "configured manifest has different images that the default manifest",
			envVariablesWithValue: []string{
				images.SkupperImageRegistryEnvKey,
				images.ConfigSyncImageEnvKey,
				images.RouterImageEnvKey},
			expectedConfiguredManifest: Manifest{
				Images: []SkupperImage{
					{
						Name: "SKUPPER_CONFIG_SYNC_IMAGE_TESTING",
					},
					{
						Name: "SKUPPER_ROUTER_IMAGE_TESTING"},
				},
			},
			expectedDefaultManifestWithEnv: Manifest{
				Images: []SkupperImage{
					{
						Name: strings.Join([]string{images.DefaultImageRegistry, images.ConfigSyncImageName}, "/"),
					},
					{
						Name: strings.Join([]string{images.DefaultImageRegistry, images.RouterImageName}, "/"),
					},
				},
				Variables: &map[string]string{
					images.SkupperImageRegistryEnvKey: "SKUPPER_IMAGE_REGISTRY_TESTING",
					images.ConfigSyncImageEnvKey:      "SKUPPER_CONFIG_SYNC_IMAGE_TESTING",
					images.RouterImageEnvKey:          "SKUPPER_ROUTER_IMAGE_TESTING",
				},
			},
		},
		{
			title:                 "configured manifest the same images that the default manifest",
			envVariablesWithValue: []string{},
			expectedConfiguredManifest: Manifest{
				Images: []SkupperImage{
					{
						Name: strings.Join([]string{images.DefaultImageRegistry, images.ConfigSyncImageName}, "/"),
					},
					{
						Name: strings.Join([]string{images.DefaultImageRegistry, images.RouterImageName}, "/"),
					},
				},
			},
			expectedDefaultManifestWithEnv: Manifest{
				Images: []SkupperImage{
					{
						Name: strings.Join([]string{images.DefaultImageRegistry, images.ConfigSyncImageName}, "/"),
					},
					{
						Name: strings.Join([]string{images.DefaultImageRegistry, images.RouterImageName}, "/"),
					},
				},
				Variables: &map[string]string{},
			},
		},
	}

	for _, c := range testcases {

		setUpEnvVariables(c.envVariablesWithValue)

		manifestManager := ManifestManager{EnableSHA: false}
		configuredManifest := manifestManager.GetConfiguredManifest()
		defaultManifest := manifestManager.GetDefaultManifestWithEnv()

		for _, expectedImage := range c.expectedConfiguredManifest.Images {

			configuredImage := getSkupperImageFromManifest(&configuredManifest, expectedImage.Name)

			assert.Check(t, configuredImage != nil)
		}

		assert.Check(t, configuredManifest.Variables == nil)

		for _, expectedImage := range c.expectedDefaultManifestWithEnv.Images {

			defaultImage := getSkupperImageFromManifest(&defaultManifest, expectedImage.Name)

			assert.Check(t, defaultImage != nil)
		}

		assert.DeepEqual(t, c.expectedDefaultManifestWithEnv.Variables, defaultManifest.Variables)

		clearEnvVariables(c.envVariablesWithValue)
	}
}

func setUpEnvVariables(variables []string) {
	for _, variable := range variables {
		err := os.Setenv(variable, strings.Join([]string{variable, "TESTING"}, "_"))
		if err != nil {
			fmt.Printf("error while setting %s", variable)
			return
		}
	}
}

func clearEnvVariables(variables []string) {
	for _, variable := range variables {
		err := os.Unsetenv(variable)
		if err != nil {
			fmt.Printf("error while unssetting %s", variable)
			return
		}
	}
}

func getSkupperImageFromManifest(m *Manifest, name string) *SkupperImage {

	for _, skImage := range m.Images {

		if skImage.Name == name {
			return &skImage
		}
	}

	return nil
}
