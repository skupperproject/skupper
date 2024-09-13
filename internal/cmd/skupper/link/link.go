package link

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/link/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/link/nonkube"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/spf13/cobra"
	"time"
)

func NewCmdLink() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "link",
		Short: "A site-to-site communication channel",
		Long:  `A site-to-site communication channel. Links serve as a transport for application connections and requests. A set of linked sites constitute a network.`,
		Example: `skupper link generate
skupper link status`,
	}

	cmd.AddCommand(CmdLinkGenerateFactory(config.GetPlatform()))
	cmd.AddCommand(CmdLinkUpdateFactory(config.GetPlatform()))
	cmd.AddCommand(CmdLinkStatusFactory(config.GetPlatform()))
	cmd.AddCommand(CmdLinkDeleteFactory(config.GetPlatform()))

	return cmd
}

func CmdLinkGenerateFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdLinkGenerate()
	nonKubeCommand := nonkube.NewCmdLinkGenerate()

	cmdLinkGenerateDesc := common.SkupperCmdDescription{
		Use:   "generate",
		Short: "Generate a new link resource in a yaml file",
		Long: `Generate a new link resource with the data needed from the target site. The resultant
output needs to be applied in the site in which we want to create the link.`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdLinkGenerateDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandLinkGenerateFlags{}
	cmd.Flags().StringVar(&cmdFlags.TlsSecret, common.FlagNameTlsSecret, "", common.FlagDescTlsSecret)
	cmd.Flags().StringVar(&cmdFlags.Cost, common.FlagNameCost, "1", common.FlagDescCost)
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "yaml", common.FlagDescOutput)
	cmd.Flags().BoolVar(&cmdFlags.GenerateCredential, common.FlagNameGenerateCredential, true, common.FlagDescGenerateCredential)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdLinkUpdateFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdLinkUpdate()
	nonKubeCommand := nonkube.NewCmdLinkUpdate()

	cmdLinkUpdateDesc := common.SkupperCmdDescription{
		Use:   "update <name>",
		Short: "Change link settings",
		Long:  "Change link settings",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdLinkUpdateDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandLinkUpdateFlags{}
	cmd.Flags().StringVar(&cmdFlags.TlsSecret, common.FlagNameTlsSecret, "", common.FlagDescTlsSecret)
	cmd.Flags().StringVar(&cmdFlags.Cost, common.FlagNameCost, "1", common.FlagDescCost)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdLinkStatusFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdLinkStatus()
	nonKubeCommand := nonkube.NewCmdLinkStatus()

	cmdLinkStatusDesc := common.SkupperCmdDescription{
		Use:     "status",
		Short:   "Display the status",
		Long:    "Display the status of links in the current site.",
		Example: "skupper link status",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdLinkStatusDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandLinkStatusFlags{}
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdLinkDeleteFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdLinkDelete()
	nonKubeCommand := nonkube.NewCmdLinkDelete()

	cmdLinkDeleteDesc := common.SkupperCmdDescription{
		Use:     "delete <name>",
		Short:   "Delete a link",
		Long:    "Delete a link by name",
		Example: "skupper site delete my-link",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdLinkDeleteDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandLinkDeleteFlags{}
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}
