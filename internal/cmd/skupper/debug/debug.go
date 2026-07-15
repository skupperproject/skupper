package debug

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/nonkube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/sweeper"
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
	cmd.AddCommand(CmdDebugSweepFactory(platform))

	return cmd
}

func CmdDebugSweepFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdConnSweeper()
	nonKubeCommand := nonkube.NewCmdConnSweeper()

	cmdDesc := common.SkupperCmdDescription{
		Use:   "sweep",
		Short: "Detect and kill idle TCP adaptor connections",
		Long: `Queries the router management API for TCP adaptor connections, identifies
connections that have been idle beyond the threshold, and force-closes them
via adminStatus=deleted.`,
		Example: "skupper debug sweep --url amqp://127.0.0.1:5672 --idle-threshold 14400",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdDesc, kubeCommand, nonKubeCommand)
	cmd.Hidden = true

	cmdFlags := common.CommandConnSweeperFlags{
		URL:           sweeper.DefaultURL,
		IdleThreshold: sweeper.DefaultIdleThreshold,
		Skmanage:      sweeper.DefaultSkmanage,
	}

	cmd.Flags().StringVar(&cmdFlags.URL, "url", sweeper.DefaultURL, "Router management URL")
	cmd.Flags().IntVar(&cmdFlags.IdleThreshold, "idle-threshold", sweeper.DefaultIdleThreshold, "Seconds with no data received before a connection is flagged as orphaned")
	cmd.Flags().BoolVar(&cmdFlags.DryRun, "dry-run", false, "List idle connections without killing them")
	cmd.Flags().StringVar(&cmdFlags.Skmanage, "skmanage", sweeper.DefaultSkmanage, "Path to the skmanage binary")

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

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
