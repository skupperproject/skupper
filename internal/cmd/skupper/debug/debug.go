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
via adminStatus=deleted.

With --list-ports it instead reports how many connections each port carries,
inbound and outbound, and closes nothing.

--port narrows either mode to the given ports. Note that closing a connection
also closes the other leg of its flow, which sits on the connector's port and
so may differ from the port swept.`,
		Example: `skupper debug sweep --idle-threshold 14400
skupper debug sweep --list-ports
skupper debug sweep --port 8080 --port 9090 --idle-threshold 14400 --execute`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdDesc, kubeCommand, nonKubeCommand)
	cmd.Hidden = true

	var cmdFlags common.CommandConnSweeperFlags

	cmd.Flags().IntVar(&cmdFlags.IdleThreshold, "idle-threshold", sweeper.DefaultIdleThreshold, "Seconds with no data received before a connection is flagged as orphaned")
	cmd.Flags().BoolVar(&cmdFlags.Execute, "execute", false, "Close the idle connections found; without this flag they are only listed")
	cmd.Flags().BoolVar(&cmdFlags.ListPorts, "list-ports", false, "List each port in use with its inbound and outbound connection counts, instead of sweeping")
	cmd.Flags().IntSliceVar(&cmdFlags.Ports, "port", nil, "Only consider connections on this port; repeat the flag for several ports (default: all ports)")

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
