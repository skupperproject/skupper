package token

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/token/kube"
	"github.com/spf13/cobra"
)

var SelectedNamespace string
var SelectedContext string
var KubeConfigPath string

func NewCmdToken() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Security mechanism for creating connections between Skupper sites.",
		Long: `A token contains connection information and necessary credentials for one Skupper 
service network to connect to another.
Issue the token on the site that was configured to allow incoming links.
Redeem the token on the other site. `,
		Example: `skupper token issue <name> ~/token.yaml`,
	}

	tokenIssueCommand := kube.NewCmdTokenIssue()
	tokenRedeemCommand := kube.NewCmdTokenRedeem()

	cmd.AddCommand(&tokenIssueCommand.CobraCmd)
	cmd.AddCommand(&tokenRedeemCommand.CobraCmd)

	//these flags are only valid for the kubernetes implementation
	cmd.PersistentFlags().StringVarP(&SelectedNamespace, "namespace", "n", "", "Set the namespace")
	cmd.PersistentFlags().StringVarP(&SelectedContext, "context", "c", "", "Set the kubeconfig context")
	cmd.PersistentFlags().StringVarP(&KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")

	return cmd
}
