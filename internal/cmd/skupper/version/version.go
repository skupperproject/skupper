package version

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/version/nonkube"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/skupperproject/skupper/internal/cmd/skupper/version/kube"

	"github.com/spf13/cobra"
)

func NewCmdVersion() *cobra.Command {

	cmd := CmdVersionFactory(config.GetPlatform())

	return cmd
}

func CmdVersionFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdVersion()
	nonKubeCommand := nonkube.NewCmdVersion()

	cmdVersionDesc := common.SkupperCmdDescription{
		Use:   "version",
		Short: "Display versions of Skupper components.",
		Long:  "Report the version of the Skupper components",
		Example: `skupper version
skupper version -o yaml > manifest.yaml`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdVersionDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandVersionFlags{}
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagVerboseOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd

}
