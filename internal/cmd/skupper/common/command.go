package common

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/spf13/cobra"
)

type SkupperCommand interface {
	NewClient(cobraCommand *cobra.Command, args []string)
	ValidateInput(args []string) []error
	InputToOptions()
	Run() error
	WaitUntil() error
}

type SkupperCmdDescription struct {
	Use     string
	Short   string
	Long    string
	Example string
}

func ConfigureCobraCommand(configuredPlatform types.Platform, description SkupperCmdDescription, kubeImpl SkupperCommand, nonKubeImpl SkupperCommand) *cobra.Command {
	var skupperCommand SkupperCommand
	var platform string

	cmd := cobra.Command{
		Use:     description.Use,
		Short:   description.Short,
		Long:    description.Long,
		Example: description.Example,
		PreRunE: func(cmd *cobra.Command, args []string) error {

			platform = string(configuredPlatform)
			if cmd.Flag("platform") != nil && cmd.Flag("platform").Value.String() != "" {
				platform = cmd.Flag("platform").Value.String()
			}

			switch platform {
			case "kubernetes":
				skupperCommand = kubeImpl
			case "podman", "docker", "systemd":
				skupperCommand = nonKubeImpl
			default:
				return fmt.Errorf("platform %q not supported", platform)
			}

			skupperCommand.NewClient(cmd, args)
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCommand.ValidateInput(args))
			skupperCommand.InputToOptions()
			utils.HandleError(skupperCommand.Run())
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			utils.HandleError(skupperCommand.WaitUntil())
		},
	}

	return &cmd
}
