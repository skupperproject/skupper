package connector

import "github.com/spf13/cobra"

func NewCmdConnector() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "connector",
		Short:   "",
		Long:    ``,
		Example: ``,
	}

	connectorCreateCommand := NewCmdConnectorCreate()
	connectorGetCommand := NewCmdConnectorGet()
	connectorUpdateCommand := NewCmdConnectorUpdate()
	connectorDeleteCommand := NewCmdConnectorDelete()

	cmd.AddCommand(&connectorCreateCommand.CobraCmd)
	cmd.AddCommand(&connectorGetCommand.CobraCmd)
	cmd.AddCommand(&connectorUpdateCommand.CobraCmd)
	cmd.AddCommand(&connectorDeleteCommand.CobraCmd)

	return cmd
}
