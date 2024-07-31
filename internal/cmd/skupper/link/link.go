package link

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/link/kube"
	"github.com/spf13/cobra"
)

var SelectedNamespace string
var SelectedContext string
var KubeConfigPath string

func NewCmdLink() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "link",
		Short: "A site-to-site communication channel",
		Long:  `A site-to-site communication channel. Links serve as a transport for application connections and requests. A set of linked sites constitute a network.`,
		Example: `skupper link generate
skupper link status`,
	}

	linkExportCommand := kube.NewCmdLinkGenerate()
	linkUpdateCommand := kube.NewCmdLinkUpdate()
	linkStatusCommand := kube.NewCmdLinkStatus()
	linkDeleteCommand := kube.NewCmdLinkDelete()

	cmd.AddCommand(&linkExportCommand.CobraCmd)
	cmd.AddCommand(&linkUpdateCommand.CobraCmd)
	cmd.AddCommand(&linkDeleteCommand.CobraCmd)
	cmd.AddCommand(&linkStatusCommand.CobraCmd)

	//these flags are only valid for the kubernetes implementation
	cmd.PersistentFlags().StringVarP(&SelectedNamespace, "namespace", "n", "", "Set the namespace")
	cmd.PersistentFlags().StringVarP(&SelectedContext, "context", "c", "", "Set the kubeconfig context")
	cmd.PersistentFlags().StringVarP(&KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")

	return cmd
}
