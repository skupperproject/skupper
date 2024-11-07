package debug

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/nonkube"

	"github.com/skupperproject/skupper/pkg/config"
	"github.com/spf13/cobra"
)

func NewCmdDebug() *cobra.Command {

	cmd := CmdDebugFactory(config.GetPlatform())

	return cmd
}

func CmdDebugFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdDebug()
	nonKubeCommand := nonkube.NewCmdDebug()

	cmdDebugDesc := common.SkupperCmdDescription{
		Use:   "debug",
		Short: "",
		Long:  "",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdDebugDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandDebugFlags{}

	//Add flags if necessary

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd

}
