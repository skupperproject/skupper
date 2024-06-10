package listener

import "github.com/spf13/cobra"

func NewCmdListener() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "listener",
		Short:   "",
		Long:    ``,
		Example: ``,
	}

	listenerCreateCommand := NewCmdListenerCreate()
	listenerGetCommand := NewCmdListenerGet()
	listenerUpdateCommand := NewCmdListenerUpdate()
	listenerDeleteCommand := NewCmdListenerDelete()

	cmd.AddCommand(&listenerCreateCommand.CobraCmd)
	cmd.AddCommand(&listenerGetCommand.CobraCmd)
	cmd.AddCommand(&listenerUpdateCommand.CobraCmd)
	cmd.AddCommand(&listenerDeleteCommand.CobraCmd)

	return cmd
}
