package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/pkg/images"
	"github.com/spf13/cobra"
)

type SkupperImage struct {
	Name       string `yaml:"name"`
	SHA        string `yaml:"sha"`
	Repository string `yaml:"repository,omitempty"`
}

func generateManifestFile() error {
	// Define a struct for the manifest file.
	manifest := struct {
		Images []SkupperImage `json:"images"`
	}{
		Images: []SkupperImage{
			{
				Name:       images.GetRouterImageName(),
				SHA:        images.GetSha(images.GetRouterImageName()),
				Repository: "https://github.com/skupperproject/skupper-router",
			},
			{
				Name:       images.GetServiceControllerImageName(),
				SHA:        images.GetSha(images.GetServiceControllerImageName()),
				Repository: "https://github.com/skupperproject/skupper",
			},
			{
				Name:       images.GetConfigSyncImageName(),
				SHA:        images.GetSha(images.GetConfigSyncImageName()),
				Repository: "https://github.com/skupperproject/skupper",
			},
			{
				Name:       images.GetFlowCollectorImageName(),
				SHA:        images.GetSha(images.GetFlowCollectorImageName()),
				Repository: "https://github.com/skupperproject/skupper",
			},
			{
				Name:       images.GetSiteControllerImageName(),
				SHA:        images.GetSha(images.GetSiteControllerImageName()),
				Repository: "https://github.com/skupperproject/skupper",
			},
			{
				Name: images.GetPrometheusServerImageName(),
				SHA:  images.GetSha(images.GetPrometheusServerImageName()),
			},
			{
				Name: images.GetOauthProxyImageName(),
				SHA:  images.GetSha(images.GetOauthProxyImageName()),
			},
		},
	}

	// Encode the manifest image list as JSON.
	manifestListJSON, err := json.MarshalIndent(manifest, "", "   ")
	if err != nil {
		return fmt.Errorf("Error encoding manifest image list: %v\n", err)

	}

	// Create a new file.
	file, err := os.Create("manifest.json")
	if err != nil {
		return fmt.Errorf("Error creating file: %v\n", err)
	}

	// Write the JSON data to the file.
	_, err = file.Write(manifestListJSON)
	if err != nil {
		return fmt.Errorf("Error writing to file: %v\n", err)
	}

	return nil
}

func main() {

	var command = &cobra.Command{
		Use: "manifest",
		Long: "This command produces a manifest detailing the images and corresponding SHAs used in Skupper. " +
			"To create this manifest file, it requires the installation of Docker.",
		Short: "generates a manifest.json file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateManifestFile()
		},
	}

	if err := command.Execute(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			panic(err)
		}
		os.Exit(1)
	}
}
