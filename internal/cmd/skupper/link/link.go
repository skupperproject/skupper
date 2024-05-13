package link

import "github.com/spf13/cobra"

func NewCmdLink() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "link",
		Short:   "",
		Long:    ``,
		Example: ``,
	}

	linkCreateCommand := NewCmdLinkCreate()
	linkGetCommand := NewCmdLinkGet()
	linkDeleteCommand := NewCmdLinkDelete()

	cmd.AddCommand(&linkCreateCommand.CobraCmd)
	cmd.AddCommand(&linkGetCommand.CobraCmd)
	cmd.AddCommand(&linkDeleteCommand.CobraCmd)

	return cmd
}
