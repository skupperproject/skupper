package manifest

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/manifest/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/manifest/nonkube"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/spf13/cobra"
)

func NewCmdManifest() *cobra.Command {

	platform := common.Platform(config.GetPlatform())
	cmd := CmdManifestFactory(platform)

	return cmd
}

func CmdManifestFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdManifest()
	nonKubeCommand := nonkube.NewCmdManifest()

	cmdManifestDesc := common.SkupperCmdDescription{
		Use:   "manifest",
		Short: "Display image versions of Skupper components.",
		Long:  "Report the image versions of the Skupper components.",
		Example: `skupper manifest
skupper manifest -o yaml > manifest.yaml`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdManifestDesc, kubeCommand, nonKubeCommand)
	cmd.Hidden = true

	cmdFlags := common.CommandVersionFlags{}
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagVerboseOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd

}
