package configs

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/images"
	"gotest.tools/v3/assert"
)

var (
	manifestComponents = []string{"router", "controller"}
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
				images.KubeAdaptorImageEnvKey,
				images.RouterImageEnvKey,
				images.ControllerImageEnvKey,
			},
			expectedConfiguredManifest: Manifest{
				Images: SkupperManifest{
					Components: []SkupperComponent{
						{
							Component: "router",
							Version:   "main",
							Images: []images.SkupperImage{
								{
									Name: "SKUPPER_KUBE_ADAPTOR_IMAGE_TESTING",
								},
								{
									Name: "SKUPPER_ROUTER_IMAGE_TESTING",
								},
							},
						},
						{
							Component: "controller",
							Version:   strings.Split(images.ControllerImageName, ":")[1],
							Images: []images.SkupperImage{
								{
									Name: "SKUPPER_CONTROLLER_IMAGE_TESTING",
								},
							},
						},
					},
				},
			},
			expectedDefaultManifestWithEnv: Manifest{
				Images: SkupperManifest{
					Components: []SkupperComponent{
						{
							Component: "router",
							Version:   "",
							Images: []images.SkupperImage{
								{
									Name: strings.Join([]string{images.DefaultImageRegistry, images.KubeAdaptorImageName}, "/"),
								},
								{
									Name: strings.Join([]string{images.DefaultImageRegistry, images.RouterImageName}, "/"),
								},
							},
						},
					},
				},
				Variables: &map[string]string{
					images.SkupperImageRegistryEnvKey: "SKUPPER_IMAGE_REGISTRY_TESTING",
					images.KubeAdaptorImageEnvKey:     "SKUPPER_KUBE_ADAPTOR_IMAGE_TESTING",
					images.RouterImageEnvKey:          "SKUPPER_ROUTER_IMAGE_TESTING",
					images.ControllerImageEnvKey:      "SKUPPER_CONTROLLER_IMAGE_TESTING",
				},
			},
		},
		{
			title:                 "configured manifest the same images that the default manifest",
			envVariablesWithValue: []string{},
			expectedConfiguredManifest: Manifest{
				Images: SkupperManifest{
					Components: []SkupperComponent{
						{
							Component: "router",
							Version:   "main",
							Images: []images.SkupperImage{
								{
									Name: strings.Join([]string{images.DefaultImageRegistry, images.KubeAdaptorImageName}, "/"),
								},
								{
									Name: strings.Join([]string{images.DefaultImageRegistry, images.RouterImageName}, "/"),
								},
							},
						},
					},
				},
			},
			expectedDefaultManifestWithEnv: Manifest{
				Images: SkupperManifest{
					Components: []SkupperComponent{
						{
							Component: "router",
							Version:   "main",
							Images: []images.SkupperImage{
								{
									Name: strings.Join([]string{images.DefaultImageRegistry, images.KubeAdaptorImageName}, "/"),
								},
								{
									Name: strings.Join([]string{images.DefaultImageRegistry, images.RouterImageName}, "/"),
								},
							},
						},
					},
				},
				Variables: &map[string]string{},
			},
		},
	}

	for _, c := range testcases {

		setUpEnvVariables(c.envVariablesWithValue)

		manifestManager := ManifestManager{Components: manifestComponents, EnableSHA: false}
		configuredManifest := manifestManager.GetConfiguredManifest()
		defaultManifest := manifestManager.GetDefaultManifestWithEnv()

		for _, expectedImage := range c.expectedConfiguredManifest.Images.Components {

			configuredImage := getSkupperImageFromManifest(&configuredManifest, expectedImage.Component)

			assert.Check(t, configuredImage != nil)
		}

		if c.expectedDefaultManifestWithEnv.Variables == nil {
			assert.Check(t, defaultManifest.Variables == nil)
		}

		for _, expectedImage := range c.expectedDefaultManifestWithEnv.Images.Components {

			defaultImage := getSkupperImageFromManifest(&defaultManifest.Images, expectedImage.Component)

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

func getSkupperImageFromManifest(m *SkupperManifest, name string) *SkupperComponent {

	for _, skImage := range m.Components {

		if skImage.Component == name {
			return &skImage
		}
	}

	return nil
}
