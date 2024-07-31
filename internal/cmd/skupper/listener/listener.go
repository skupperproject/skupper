package listener

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/listener/kube"
	"github.com/spf13/cobra"
)

var SelectedNamespace string
var SelectedContext string
var KubeConfigPath string

func NewCmdListener() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "listener",
		Short: "Binds target workloads in the local site to target workloads in remote sites.",
		Long:  `A listener is a connection endpoint in the local site and binds to target workloads in remote sites`,
		Example: `skupper listener create my-listener 8080
skupper listener status my-listener`,
	}

	listenerCreateCommand := kube.NewCmdListenerCreate()
	listenerStatusCommand := kube.NewCmdListenerStatus()
	listenerUpdateCommand := kube.NewCmdListenerUpdate()
	listenerDeleteCommand := kube.NewCmdListenerDelete()

	cmd.AddCommand(&listenerCreateCommand.CobraCmd)
	cmd.AddCommand(&listenerStatusCommand.CobraCmd)
	cmd.AddCommand(&listenerUpdateCommand.CobraCmd)
	cmd.AddCommand(&listenerDeleteCommand.CobraCmd)

	//these flags are only valid for the kubernetes implementation
	cmd.PersistentFlags().StringVarP(&SelectedNamespace, "namespace", "n", "", "Set the namespace")
	cmd.PersistentFlags().StringVarP(&SelectedContext, "context", "c", "", "Set the kubeconfig context")
	cmd.PersistentFlags().StringVarP(&KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")

	return cmd
}
