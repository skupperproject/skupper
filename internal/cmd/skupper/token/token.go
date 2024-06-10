package token

import (
	"github.com/spf13/cobra"
)

func NewCmdToken() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Security mechanism for creating connections between Skupper sites.",
		Long: `A token contains connection information and necessary credentials for one Skupper 
service network to connect to another.`,
		Example: `skupper token create ~/token.yaml`,
	}

	tokenCreateCommand := NewCmdTokenCreate()

	cmd.AddCommand(&tokenCreateCommand.CobraCmd)

	return cmd
}
