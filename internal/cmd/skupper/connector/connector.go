package connector

import (
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/connector/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/connector/nonkube"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/spf13/cobra"
)

func NewCmdConnector() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "connector",
		Short: "Binds target workloads in the local site to listeners in remote sites.",
		Long:  `A connector is a endpoint in the local site and binds to listeners in remote sites`,
		Example: `skupper connector create my-connector 8080
skupper connector status my-connector`,
	}

	cmd.AddCommand(CmdConnectorCreateFactory(config.GetPlatform()))
	cmd.AddCommand(CmdConnectorStatusFactory(config.GetPlatform()))
	cmd.AddCommand(CmdConnectorUpdateFactory(config.GetPlatform()))
	cmd.AddCommand(CmdConnectorDeleteFactory(config.GetPlatform()))
	cmd.AddCommand(CmdConnectorGenerateFactory(config.GetPlatform()))

	return cmd
}

func CmdConnectorCreateFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdConnectorCreate()
	nonKubeCommand := nonkube.NewCmdConnectorCreate()

	cmdConnectorCreateDesc := common.SkupperCmdDescription{
		Use:   "create <name> <port>",
		Short: "create a connector",
		Long:  "Clients at this site use the connector host and port to establish connections to the remote service.",
		Example: `skupper connector create database 5432
skupper connector create backend 8080 --workload deployment/backend`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdConnectorCreateDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandConnectorCreateFlags{}

	cmd.Flags().StringVarP(&cmdFlags.RoutingKey, common.FlagNameRoutingKey, "r", "", common.FlagDescRoutingKey)
	cmd.Flags().StringVar(&cmdFlags.Host, common.FlagNameHost, "", common.FlagDescHost)
	cmd.Flags().StringVar(&cmdFlags.TlsCredentials, common.FlagNameTlsCredentials, "", common.FlagDescTlsCredentials)
	cmd.Flags().StringVar(&cmdFlags.ConnectorType, common.FlagNameConnectorType, "tcp", common.FlagDescConnectorType)
	if configuredPlatform == types.PlatformKubernetes {
		cmd.Flags().BoolVarP(&cmdFlags.IncludeNotReadyPods, common.FlagNameIncludeNotReadyPods, "i", false, common.FlagDescIncludeNotRead)
		cmd.Flags().StringVarP(&cmdFlags.Selector, common.FlagNameSelector, "s", "", common.FlagDescSelector)
		cmd.Flags().StringVarP(&cmdFlags.Workload, common.FlagNameWorkload, "w", "", common.FlagDescWorkload)
		cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)
		cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "configured", common.FlagDescWait)
	}

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdConnectorStatusFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdConnectorStatus()
	nonKubeCommand := nonkube.NewCmdConnectorStatus()

	cmdConnectorStatusDesc := common.SkupperCmdDescription{
		Use:     "status <name>",
		Short:   "get status of connectors",
		Long:    "Display status of all connectors or a specific connector",
		Example: "skupper connector status backend",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdConnectorStatusDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandConnectorStatusFlags{}
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameConnectorStatusOutput, "o", "", common.FlagDescConnectorStatusOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdConnectorUpdateFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdConnectorUpdate()
	nonKubeCommand := nonkube.NewCmdConnectorUpdate()

	cmdConnectorUpdateDesc := common.SkupperCmdDescription{
		Use:   "update <name>",
		Short: "update a connector",
		Long: `Clients at this site use the connector host and port to establish connections to the remote service.
	The user can change port, host name, TLS secret, selector, connector type and routing key`,
		Example: "skupper connector update database --host mysql --port 3306",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdConnectorUpdateDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandConnectorUpdateFlags{}

	cmd.Flags().StringVarP(&cmdFlags.RoutingKey, common.FlagNameRoutingKey, "r", "", common.FlagDescRoutingKey)
	cmd.Flags().StringVar(&cmdFlags.Host, common.FlagNameHost, "", common.FlagDescHost)
	cmd.Flags().StringVar(&cmdFlags.TlsCredentials, common.FlagNameTlsCredentials, "", common.FlagDescTlsCredentials)
	cmd.Flags().StringVar(&cmdFlags.ConnectorType, common.FlagNameConnectorType, "tcp", common.FlagDescConnectorType)
	cmd.Flags().IntVar(&cmdFlags.Port, common.FlagNameConnectorPort, 0, common.FlagDescConnectorPort)
	if configuredPlatform == types.PlatformKubernetes {
		cmd.Flags().BoolVarP(&cmdFlags.IncludeNotReadyPods, common.FlagNameIncludeNotReadyPods, "i", false, common.FlagDescIncludeNotRead)
		cmd.Flags().StringVarP(&cmdFlags.Selector, common.FlagNameSelector, "s", "", common.FlagDescSelector)
		cmd.Flags().StringVarP(&cmdFlags.Workload, common.FlagNameWorkload, "w", "", common.FlagDescWorkload)
		cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)
		cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "configured", common.FlagDescWait)
	}

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdConnectorDeleteFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdConnectorDelete()
	nonKubeCommand := nonkube.NewCmdConnectorDelete()

	cmdConnectorDeleteDesc := common.SkupperCmdDescription{
		Use:     "delete <name>",
		Short:   "delete a connector",
		Long:    "Delete a connector <name>",
		Example: "skupper connector delete database",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdConnectorDeleteDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandConnectorDeleteFlags{}
	cmd.Flags().DurationVarP(&cmdFlags.Timeout, common.FlagNameTimeout, "t", 60*time.Second, common.FlagDescTimeout)
	cmd.Flags().BoolVar(&cmdFlags.Wait, common.FlagNameWait, true, common.FlagDescDeleteWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdConnectorGenerateFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdConnectorGenerate()
	nonKubeCommand := nonkube.NewCmdConnectorGenerate()

	cmdConnectorGenerateDesc := common.SkupperCmdDescription{
		Use:   "generate <name> <port>",
		Short: "Generate a connector resource and output it to a file or screen",
		Long:  "Clients at this site use the connector host and port to establish connections to the remote service.",
		Example: `skupper connector generate database 5432
skupper connector generate backend 8080 --workload deployment/backend`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdConnectorGenerateDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandConnectorGenerateFlags{}

	cmd.Flags().StringVarP(&cmdFlags.RoutingKey, common.FlagNameRoutingKey, "r", "", common.FlagDescRoutingKey)
	cmd.Flags().StringVar(&cmdFlags.Host, common.FlagNameHost, "", common.FlagDescHost)
	cmd.Flags().StringVar(&cmdFlags.TlsCredentials, common.FlagNameTlsCredentials, "", common.FlagDescTlsCredentials)
	cmd.Flags().StringVar(&cmdFlags.ConnectorType, common.FlagNameConnectorType, "tcp", common.FlagDescConnectorType)
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "yaml", common.FlagDescOutput)
	if configuredPlatform == types.PlatformKubernetes {
		cmd.Flags().BoolVarP(&cmdFlags.IncludeNotReadyPods, common.FlagNameIncludeNotReadyPods, "i", false, common.FlagDescIncludeNotRead)
		cmd.Flags().StringVarP(&cmdFlags.Selector, common.FlagNameSelector, "s", "", common.FlagDescSelector)
		cmd.Flags().StringVarP(&cmdFlags.Workload, common.FlagNameWorkload, "w", "", common.FlagDescWorkload)
	}

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}
