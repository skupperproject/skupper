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
	cmd.AddCommand(CmdDebugConnFactory(platform))

	return cmd
}

func CmdDebugConnFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdConnSweeper()
	nonKubeCommand := nonkube.NewCmdConnSweeper()

	cmdDesc := common.SkupperCmdDescription{
		Use:   "conn",
		Short: "Inspect and manage TCP adaptor connections",
		Long: `Queries the router management API for TCP adaptor connections, correlates
them with listener/connector resources (routing key, resource name), and can
identify connections idle beyond a threshold and force-close them via
adminStatus=deleted.

With --list-ports it instead reports how many connections each port carries,
inbound and outbound, plus the correlated routing key and resource, and closes
nothing.

--port, --state, and --routing-key narrow either mode. Note that closing a
connection also closes the other leg of its flow, which sits on the
connector's port and so may differ from the port swept.

"sweep" remains as an alias for compatibility.`,
		Example: `skupper debug conn --idle-threshold 14400
skupper debug conn --list-ports
skupper debug conn --state ESTAB --routing-key echo:8080
skupper debug conn --port 8080 --port 9090 --idle-threshold 14400 --execute
skupper debug conn --output json --list-ports`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdDesc, kubeCommand, nonKubeCommand)
	cmd.Hidden = true
	cmd.Aliases = []string{"sweep"}

	var cmdFlags common.CommandConnSweeperFlags

	cmd.Flags().IntVar(&cmdFlags.IdleThreshold, "idle-threshold", sweeper.DefaultIdleThreshold, "Seconds with no data received before a connection is flagged as orphaned")
	cmd.Flags().BoolVar(&cmdFlags.Execute, "execute", false, "Close the idle connections found; without this flag they are only listed")
	cmd.Flags().BoolVar(&cmdFlags.ListPorts, "list-ports", false, "List each port in use with its inbound and outbound connection counts, instead of sweeping")
	cmd.Flags().IntSliceVar(&cmdFlags.Ports, "port", nil, "Only consider connections on this port; repeat the flag for several ports (default: all ports)")
	cmd.Flags().StringSliceVar(&cmdFlags.States, "state", nil, "Only consider connections whose kernel socket is in this TCP state (ss names: ESTAB, FIN-WAIT-2, …); repeat or comma-separate")
	cmd.Flags().StringSliceVar(&cmdFlags.RoutingKeys, "routing-key", nil, "Only consider connections correlated to this routing key; repeat or comma-separate")
	cmd.Flags().StringVar(&cmdFlags.Output, "output", sweeper.OutputText, "Output format: text or json")

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

// CmdDebugSweepFactory is retained for callers that still reference the old name.
func CmdDebugSweepFactory(configuredPlatform common.Platform) *cobra.Command {
	return CmdDebugConnFactory(configuredPlatform)
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
