package main

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/utils/configs"
	"os"

	"github.com/spf13/cobra"
)

func main() {

	var command = &cobra.Command{
		Use: "manifest",
		Long: "This command produces a manifest detailing the images and corresponding SHAs used in Skupper. " +
			"To create this manifest file, it requires the installation of Docker.",
		Short: "generates a manifest.json file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestManager := configs.ManifestManager{EnableSHA: true}
			return manifestManager.CreateFile(manifestManager.GetConfiguredManifest())

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
