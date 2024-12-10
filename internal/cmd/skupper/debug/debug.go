package debug

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/nonkube"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/spf13/cobra"
)

func NewCmdDebug() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Create a tarball containing various files with the site details",
		Long: `Create a tarball including site resources and status; component versions, config files, 
	and logs; and info about the environment where Skupper is running`,
		Example: "skupper debug dump <filename>",
	}

	cmd.AddCommand(CmdDebugDumpFactory(config.GetPlatform()))

	return cmd
}

func CmdDebugDumpFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdDebug()
	nonKubeCommand := nonkube.NewCmdDebug()

	cmdDebugDesc := common.SkupperCmdDescription{
		Use:     "dump <fileName>",
		Short:   "Create a tarball",
		Long:    "Create a tarball including site resources and status",
		Example: "skupper debug dump <filename>",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdDebugDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandDebugFlags{}

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd

}
