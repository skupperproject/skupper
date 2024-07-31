package link

import "github.com/spf13/cobra"

var SelectedNamespace string
var SelectedContext string
var KubeConfigPath string

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

	//these flags are only valid for the kubernetes implementation
	cmd.PersistentFlags().StringVarP(&SelectedNamespace, "namespace", "n", "", "Set the namespace")
	cmd.PersistentFlags().StringVarP(&SelectedContext, "context", "c", "", "Set the kubeconfig context")
	cmd.PersistentFlags().StringVarP(&KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")

	return cmd
}
