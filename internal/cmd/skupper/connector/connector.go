package connector

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/connector/kube"
	"github.com/spf13/cobra"
)

var SelectedNamespace string
var SelectedContext string
var KubeConfigPath string

func NewCmdConnector() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "connector",
		Short: "Binds target workloads in the local site to listeners in remote sites.",
		Long:  `A connector is a endpoint in the local site and binds to listeners in remote sites`,
		Example: `skupper connector create my-connector 8080
skupper connector status my-connector`,
	}

	connectorCreateCommand := kube.NewCmdConnectorCreate()
	connectorStatusCommand := kube.NewCmdConnectorStatus()
	connectorUpdateCommand := kube.NewCmdConnectorUpdate()
	connectorDeleteCommand := kube.NewCmdConnectorDelete()

	cmd.AddCommand(&connectorCreateCommand.CobraCmd)
	cmd.AddCommand(&connectorStatusCommand.CobraCmd)
	cmd.AddCommand(&connectorUpdateCommand.CobraCmd)
	cmd.AddCommand(&connectorDeleteCommand.CobraCmd)

	//these flags are only valid for the kubernetes implementation
	cmd.PersistentFlags().StringVarP(&SelectedNamespace, "namespace", "n", "", "Set the namespace")
	cmd.PersistentFlags().StringVarP(&SelectedContext, "context", "c", "", "Set the kubeconfig context")
	cmd.PersistentFlags().StringVarP(&KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")

	return cmd
}
