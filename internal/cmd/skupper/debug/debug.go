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
	cmd.AddCommand(CmdDebugCertFactory(platform))
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
them with listener/connector resources (routing key, resource name), and lists
matching connections (idle time, TCP state, routing key). Connections idle
beyond --idle-threshold can be force-closed with --execute via
adminStatus=deleted.

With --list-ports it instead reports how many connections each port carries,
inbound and outbound, plus the correlated routing key and resource, and closes
nothing.

--port, --state, and --routing-key narrow either mode. --port is the
router-side port shown by --list-ports (on Kubernetes this is often an
allocated port such as 1024, not Listener spec.port). Note that closing a
connection also closes the other leg of its flow, which sits on the
connector's port and so may differ from the port swept.

"sweep" remains as an alias for compatibility.`,
		Example: `skupper debug conn
skupper debug conn --list-ports
skupper debug conn --state ESTAB --routing-key echo:8080
skupper debug conn --port 1024 --idle-threshold 14400 --execute
skupper debug conn --output json --list-ports`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdDesc, kubeCommand, nonKubeCommand)
	cmd.Aliases = []string{"sweep"}

	var cmdFlags common.CommandConnSweeperFlags

	cmd.Flags().IntVar(&cmdFlags.IdleThreshold, "idle-threshold", sweeper.DefaultIdleThreshold, "Seconds with no data received before a connection is flagged as orphaned")
	cmd.Flags().BoolVar(&cmdFlags.Execute, "execute", false, "Close idle connections matching the filters; without this flag matching connections are listed")
	cmd.Flags().BoolVar(&cmdFlags.ListPorts, "list-ports", false, "List each port in use with its inbound and outbound connection counts, instead of sweeping")
	cmd.Flags().IntSliceVar(&cmdFlags.Ports, "port", nil, "Only consider connections on this router-side port (see --list-ports); repeat the flag for several ports (default: all ports)")
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

func CmdDebugCertFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdDebugCert()
	nonKubeCommand := nonkube.NewCmdDebugCert()

	cmdDesc := common.SkupperCmdDescription{
		Use:   "cert-inspect [name]",
		Short: "Inspect X.509 certificates in use by Skupper",
		Long: `Decode and display key fields of X.509 certificates managed by Skupper,
including subject, issuer, validity period, SANs, and public key information.

Without a name, all certificates in the namespace are listed. With a name,
detailed information for that certificate is shown.`,
		Example: `skupper debug cert-inspect
skupper debug cert-inspect skupper-local-server
skupper debug cert-inspect --file /path/to/tls.crt
skupper debug cert-inspect -o yaml`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandDebugCertFlags{}
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)
	cmd.Flags().StringVar(&cmdFlags.File, "file", "", "Inspect a local PEM-encoded certificate file")

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
