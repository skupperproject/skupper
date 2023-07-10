package main

import (
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/pkg/images"
	"os"
)

type SkupperImage struct {
	Name       string `yaml:"name"`
	SHA        string `yaml:"sha"`
	Repository string `yaml:"repository,omitempty"`
}

func main() {

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
				Name: images.GetPrometheusServerImageName(),
				SHA:  images.GetSha(images.GetPrometheusServerImageName()),
			},
		},
	}

	// Encode the manifest image list as JSON.
	manifestListJSON, err := json.MarshalIndent(manifest, "", "   ")
	if err != nil {
		panic(fmt.Errorf("Error encoding manifest image list: %v\n", err))

	}

	// Create a new file.
	file, err := os.Create("manifest.json")
	if err != nil {
		panic(fmt.Errorf("Error creating file: %v\n", err))
	}

	// Write the JSON data to the file.
	_, err = file.Write(manifestListJSON)
	if err != nil {
		panic(fmt.Errorf("Error writing to file: %v\n", err))
	}
}
