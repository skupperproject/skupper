package main

import (
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

func NewCmdSwitch() *cobra.Command {
	validPlatforms := []string{"kubernetes", "podman", "-"}
	cmd := &cobra.Command{
		Use:       "switch <platform>",
		Short:     fmt.Sprintf("Select the platform to manage (valid platforms: %s)", strings.Join(validPlatforms, ", ")),
		ValidArgs: validPlatforms,
		Args:      cobra.OnlyValidArgs,
		Example: `
	# Display selected platform
	skupper switch

	# Switch to kubernetes
	skupper switch kubernetes

	# Switch to podman
	skupper switch podman

	# Switch back to the previous platform
	skupper switch -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			currentPlatform := config.GetPlatform()
			if len(args) == 0 {
				fmt.Printf("%s\n", currentPlatform)
				return nil
			}
			selectedPlatform := args[0]
			p := &config.PlatformInfo{}
			if err := p.Load(); err != nil {
				return err
			}
			if selectedPlatform == "-" {
				selectedPlatform = utils.DefaultStr(string(p.Previous), string(currentPlatform))
				fmt.Printf("Switched to: %s\n", selectedPlatform)
			}
			return p.Update(types.Platform(selectedPlatform))
		},
	}

	return cmd
}
