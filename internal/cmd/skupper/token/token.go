package token

import (
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/token/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/token/nonkube"
	"github.com/skupperproject/skupper/internal/config"

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

	platform := common.Platform(config.GetPlatform())
	cmd.AddCommand(CmdTokenIssueFactory(platform))
	cmd.AddCommand(CmdTokenRedeemFactory(platform))

	return cmd
}

func CmdTokenIssueFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdTokenIssue()
	nonKubeCommand := nonkube.NewCmdTokenIssue()

	cmdTokenIssueDesc := common.SkupperCmdDescription{
		Use:     "issue <fileName>",
		Short:   "issue a token",
		Long:    "Issue a token file redeemable for a link to the current site.",
		Example: "skupper token issue ~/token1.yaml",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdTokenIssueDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandTokenIssueFlags{}

	cmd.Flags().IntVarP(&cmdFlags.RedemptionsAllowed, common.FlagNameRedemptionsAllowed, "r", 1, common.FlagDescRedemptionsAllowed)
	cmd.Flags().DurationVarP(&cmdFlags.ExpirationWindow, common.FlagNameExpirationWindow, "e", 15*time.Minute, common.FlagDescExpirationWindow)
	cmd.Flags().DurationVarP(&cmdFlags.Timeout, common.FlagNameTimeout, "t", 60*time.Second, common.FlagDescTimeout)
	cmd.Flags().StringVar(&cmdFlags.Cost, common.FlagNameCost, "1", common.FlagDescCost)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdTokenRedeemFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdTokenRedeem()
	nonKubeCommand := nonkube.NewCmdTokenRedeem()

	cmdTokenRedeemDesc := common.SkupperCmdDescription{
		Use:     "redeem <filename>",
		Short:   "redeem a token",
		Long:    "Redeem a token file in order to create a link to a remote site.",
		Example: "skupper token redeem ~/token1.yaml",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdTokenRedeemDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandTokenRedeemFlags{}

	cmd.Flags().DurationVarP(&cmdFlags.Timeout, common.FlagNameTimeout, "t", 60*time.Second, common.FlagDescTimeout)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}
