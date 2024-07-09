package token

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/token/kube"
	"github.com/spf13/cobra"
)

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

	return cmd
}
