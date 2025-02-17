package link

import (
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/link/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/link/nonkube"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/spf13/cobra"
)

func NewCmdLink() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "link",
		Short: "A site-to-site communication channel",
		Long:  `A site-to-site communication channel. Links serve as a transport for application connections and requests. A set of linked sites constitute a network.`,
		Example: `skupper link generate
skupper link status`,
	}
	platform := common.Platform(config.GetPlatform())
	cmd.AddCommand(CmdLinkGenerateFactory(platform))
	cmd.AddCommand(CmdLinkUpdateFactory(platform))
	cmd.AddCommand(CmdLinkStatusFactory(platform))
	cmd.AddCommand(CmdLinkDeleteFactory(platform))

	return cmd
}

func CmdLinkGenerateFactory(configuredPlatform common.Platform) *cobra.Command {
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
	cmd.Flags().StringVar(&cmdFlags.TlsCredentials, common.FlagNameTlsCredentials, "", common.FlagDescTlsCredentials)
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

func CmdLinkUpdateFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdLinkUpdate()
	nonKubeCommand := nonkube.NewCmdLinkUpdate()

	cmdLinkUpdateDesc := common.SkupperCmdDescription{
		Use:   "update <name>",
		Short: "Change link settings",
		Long:  "Change link settings",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdLinkUpdateDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandLinkUpdateFlags{}
	cmd.Flags().StringVar(&cmdFlags.TlsCredentials, common.FlagNameTlsCredentials, "", common.FlagDescTlsCredentials)
	cmd.Flags().StringVar(&cmdFlags.Cost, common.FlagNameCost, "1", common.FlagDescCost)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)
	if configuredPlatform == common.PlatformKubernetes {
		cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "ready", common.FlagDescWait)
	}

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdLinkStatusFactory(configuredPlatform common.Platform) *cobra.Command {
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

func CmdLinkDeleteFactory(configuredPlatform common.Platform) *cobra.Command {
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
	if configuredPlatform == common.PlatformKubernetes {
		cmd.Flags().BoolVar(&cmdFlags.Wait, common.FlagNameWait, true, common.FlagDescDeleteWait)
	}

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}
