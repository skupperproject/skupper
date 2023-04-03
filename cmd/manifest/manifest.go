package main

import (
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/spf13/cobra"
	"os"
)

type SkupperImage struct {
	Name       string `yaml:"name"`
	Repository string `yaml:"repository,omitempty"`
}

func generateManifestFile() error {

	// Define an array of images.
	skupperImages := []SkupperImage{
		{
			Name:       images.GetRouterImageName(),
			Repository: "https://github.com/skupperproject/skupper-router",
		},
		{
			Name:       images.GetServiceControllerImageName(),
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       images.GetConfigSyncImageName(),
			Repository: "https://github.com/skupperproject/skupper",
		},
		{
			Name:       images.GetFlowCollectorImageName(),
			Repository: "https://github.com/skupperproject/skupper",
		},
	}

	// Define a struct for the manifest file.
	manifest := struct {
		Images []SkupperImage `json:"images"`
	}{
		Images: skupperImages,
	}

	// Encode the manifest image list as JSON.
	manifestListJSON, err := json.MarshalIndent(manifest, "", "   ")
	if err != nil {
		return fmt.Errorf("Error encoding manifest image list: %v\n", err)

	}

	// Create a new file.
	file, err := os.Create("manifest.json")
	if err != nil {
		fmt.Errorf("Error creating file: %v\n", err)
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
		Use:   "manifest",
		Short: "generates a manifest.json file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateManifestFile()
		},
	}

	command.Execute()

	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
