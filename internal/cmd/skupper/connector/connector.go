package connector

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/connector/kube"
	"github.com/spf13/cobra"
)

func NewCmdConnector() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "connector",
		Short:   "",
		Long:    ``,
		Example: ``,
	}

	connectorCreateCommand := kube.NewCmdConnectorCreate()
	connectorStatusCommand := kube.NewCmdConnectorStatus()
	connectorUpdateCommand := kube.NewCmdConnectorUpdate()
	connectorDeleteCommand := kube.NewCmdConnectorDelete()

	cmd.AddCommand(&connectorCreateCommand.CobraCmd)
	cmd.AddCommand(&connectorStatusCommand.CobraCmd)
	cmd.AddCommand(&connectorUpdateCommand.CobraCmd)
	cmd.AddCommand(&connectorDeleteCommand.CobraCmd)

	return cmd
}
