package version

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/version/nonkube"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/skupperproject/skupper/internal/cmd/skupper/version/kube"

	"github.com/spf13/cobra"
)

func NewCmdVersion() *cobra.Command {

	platform := common.Platform(config.GetPlatform())
	cmd := CmdVersionFactory(platform)

	return cmd
}

func CmdVersionFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdVersion()
	nonKubeCommand := nonkube.NewCmdVersion()

	cmdVersionDesc := common.SkupperCmdDescription{
		Use:   "image-versions",
		Short: "Display image versions of Skupper components.",
		Long:  "Report the image versions of the Skupper components.",
		Example: `skupper image-versions
skupper image-versions -o yaml > manifest.yaml`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdVersionDesc, kubeCommand, nonKubeCommand)
	cmd.Hidden = true

	cmdFlags := common.CommandVersionFlags{}
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagVerboseOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd

}
