package main

import (
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/pkg/images"
	"os"
	"os/exec"
	"strings"
)

type SkupperImage struct {
	Name       string `yaml:"name"`
	SHA        string `yaml:"sha"`
	Repository string `yaml:"repository,omitempty"`
}

func getSha(imageName string) string {
	// Pull the image
	pullCmd := exec.Command("docker", "pull", imageName)
	if err := pullCmd.Run(); err != nil {
		fmt.Printf("Error pulling image: %v", err)
		return err.Error()
	}

	// Get the image digest
	digestCmd := exec.Command("docker", "inspect", "--format='{{index .RepoDigests 0}}'", imageName)
	digestBytes, err := digestCmd.Output()
	if err != nil {
		fmt.Printf("Error getting image digest: %v", err)
		return err.Error()
	}

	imageWithoutTag := strings.Split(imageName, ":")[0]

	// Extract and print the digest
	parsedDigest := strings.ReplaceAll(strings.ReplaceAll(string(digestBytes), "'", ""), "\n", "")
	digest := strings.TrimPrefix(strings.Trim(parsedDigest, "'"), imageWithoutTag+"@")

	return digest
}

func main() {

	// Define a struct for the manifest file.
	manifest := struct {
		Images []SkupperImage `json:"images"`
	}{
		Images: []SkupperImage{
			{
				Name:       images.GetRouterImageName(),
				SHA:        getSha(images.GetRouterImageName()),
				Repository: "https://github.com/skupperproject/skupper-router",
			},
			{
				Name:       images.GetServiceControllerImageName(),
				SHA:        getSha(images.GetServiceControllerImageName()),
				Repository: "https://github.com/skupperproject/skupper",
			},
			{
				Name:       images.GetConfigSyncImageName(),
				SHA:        getSha(images.GetConfigSyncImageName()),
				Repository: "https://github.com/skupperproject/skupper",
			},
			{
				Name:       images.GetFlowCollectorImageName(),
				SHA:        getSha(images.GetFlowCollectorImageName()),
				Repository: "https://github.com/skupperproject/skupper",
			},
			{
				Name: images.GetPrometheusServerImageName(),
				SHA:  getSha(images.GetPrometheusServerImageName()),
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
