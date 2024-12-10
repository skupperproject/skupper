package listener

import (
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/listener/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/listener/nonkube"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/spf13/cobra"
)

func NewCmdListener() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "listener",
		Short: "Binds a connection endpoint in the local site to target workloads in remote sites.",
		Long:  `A listener is a connection endpoint in the local site and binds to target workloads in remote sites`,
		Example: `skupper listener create my-listener 8080
skupper listener status my-listener`,
	}

	cmd.AddCommand(CmdListenerCreateFactory(config.GetPlatform()))
	cmd.AddCommand(CmdListenerStatusFactory(config.GetPlatform()))
	cmd.AddCommand(CmdListenerUpdateFactory(config.GetPlatform()))
	cmd.AddCommand(CmdListenerDeleteFactory(config.GetPlatform()))

	return cmd
}

func CmdListenerCreateFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdListenerCreate()
	nonKubeCommand := nonkube.NewCmdListenerCreate()

	cmdListenerCreateDesc := common.SkupperCmdDescription{
		Use:     "create <name> <port>",
		Short:   "create a listener",
		Long:    "Clients at this site use the listener host and port to establish connections to the remote service.",
		Example: "skupper listener create database 5432",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdListenerCreateDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandListenerCreateFlags{}

	cmd.Flags().StringVarP(&cmdFlags.RoutingKey, common.FlagNameRoutingKey, "r", "", common.FlagDescRoutingKey)
	cmd.Flags().StringVar(&cmdFlags.Host, common.FlagNameListenerHost, "", common.FlagDescListenerHost)
	cmd.Flags().StringVar(&cmdFlags.TlsCredentials, common.FlagNameTlsCredentials, "", common.FlagDescTlsCredentials)
	cmd.Flags().StringVar(&cmdFlags.ListenerType, common.FlagNameListenerType, "tcp", common.FlagDescListenerType)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)
	cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "configured", common.FlagDescWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	if configuredPlatform != types.PlatformKubernetes {
		cmd.Flags().MarkHidden(common.FlagNameTimeout)
		cmd.Flags().MarkHidden(common.FlagNameWait)
	}

	return cmd
}

func CmdListenerUpdateFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdListenerUpdate()
	nonKubeCommand := nonkube.NewCmdListenerUpdate()

	cmdListenerUpdateDesc := common.SkupperCmdDescription{
		Use:   "update <name>",
		Short: "update a listener",
		Long: `Clients at this site use the listener host and port to establish connections to the remote service.
	The user can change port, host name, TLS credentials, listener type and routing key`,
		Example: "skupper listener update database --host mysql --port 3306",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdListenerUpdateDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandListenerUpdateFlags{}

	cmd.Flags().StringVarP(&cmdFlags.RoutingKey, common.FlagNameRoutingKey, "r", "", common.FlagDescRoutingKey)
	cmd.Flags().StringVar(&cmdFlags.Host, common.FlagNameListenerHost, "", common.FlagDescListenerHost)
	cmd.Flags().StringVarP(&cmdFlags.TlsCredentials, common.FlagNameTlsCredentials, "t", "", common.FlagDescTlsCredentials)
	cmd.Flags().StringVar(&cmdFlags.ListenerType, common.FlagNameListenerType, "tcp", common.FlagDescListenerType)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)
	cmd.Flags().IntVar(&cmdFlags.Port, common.FlagNameListenerPort, 0, common.FlagDescListenerPort)
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)
	cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "configured", common.FlagDescWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	if configuredPlatform != types.PlatformKubernetes {
		cmd.Flags().MarkHidden(common.FlagNameTimeout)
		cmd.Flags().MarkHidden(common.FlagNameWait)
	}

	return cmd
}

func CmdListenerStatusFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdListenerStatus()
	nonKubeCommand := nonkube.NewCmdListenerStatus()

	cmdListenerStatusDesc := common.SkupperCmdDescription{
		Use:     "status <name>",
		Short:   "get status of listeners",
		Long:    "Display status of all listeners or a specific listener",
		Example: "skupper listener status backend",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdListenerStatusDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandListenerStatusFlags{}

	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdListenerDeleteFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdListenerDelete()
	nonKubeCommand := nonkube.NewCmdListenerDelete()

	cmdListenerDeleteDesc := common.SkupperCmdDescription{
		Use:     "delete <name>",
		Short:   "delete a listener",
		Long:    "Delete a listener <name>",
		Example: "skupper listener delete database",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdListenerDeleteDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandListenerDeleteFlags{}

	cmd.Flags().DurationVarP(&cmdFlags.Timeout, common.FlagNameTimeout, "t", 60*time.Second, common.FlagDescTimeout)
	cmd.Flags().BoolVar(&cmdFlags.Wait, common.FlagNameWait, true, common.FlagDescDeleteWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	if configuredPlatform != types.PlatformKubernetes {
		cmd.Flags().MarkHidden(common.FlagNameTimeout)
		cmd.Flags().MarkHidden(common.FlagNameWait)
	}

	return cmd
}
