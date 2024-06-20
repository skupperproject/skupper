package listener

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/listener/kube"
	"github.com/spf13/cobra"
)

func NewCmdListener() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "listener",
		Short:   "",
		Long:    ``,
		Example: ``,
	}

	listenerCreateCommand := kube.NewCmdListenerCreate()
	listenerStatusCommand := kube.NewCmdListenerStatus()
	listenerUpdateCommand := kube.NewCmdListenerUpdate()
	listenerDeleteCommand := kube.NewCmdListenerDelete()

	cmd.AddCommand(&listenerCreateCommand.CobraCmd)
	cmd.AddCommand(&listenerStatusCommand.CobraCmd)
	cmd.AddCommand(&listenerUpdateCommand.CobraCmd)
	cmd.AddCommand(&listenerDeleteCommand.CobraCmd)

	return cmd
}
