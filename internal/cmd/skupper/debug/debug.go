package debug

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/nonkube"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/spf13/cobra"
)

func NewCmdDebug() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "debug",
		Short:   "debug site details",
		Long:    "debug site details",
		Example: "skupper debug dump <filename>",
	}
	platform := common.Platform(config.GetPlatform())
	cmd.AddCommand(CmdDebugDumpFactory(platform))
	cmd.AddCommand(CmdDebugMentatFactory(platform))

	return cmd
}

func CmdDebugDumpFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdDebug()
	nonKubeCommand := nonkube.NewCmdDebug()

	cmdDebugDesc := common.SkupperCmdDescription{
		Use:   "dump <fileName>",
		Short: "Create a tarball containing various files with the site details",
		Long: `Create a tarball including site resources and status; component versions, config files, 
	and logs; and info about the environment where Skupper is running`,
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

func CmdDebugMentatFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdDebugMentat()
	nonKubeCommand := nonkube.NewCmdDebugMentat()

	desc := common.SkupperCmdDescription{
		Use:     "mentat [dumpfile]",
		Short:   "Analyze connectivity from a debug dump using mentat",
		Long:    "Processes a skupper debug dump and prints connectivity report.",
		Example: "skupper debug mentat my-dump.tar.gz\nskupper debug mentat my-dump.tar.gz --time \"2025-05-11 14:30:00\"",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, desc, kubeCommand, nonKubeCommand)

	// Add the --time flag
	cmd.Flags().StringP("time", "t", "", "Check connectivity at a specific time (format: YYYY-MM-DD HH:MM:SS)")

	return cmd
}
