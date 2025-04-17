/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

import (
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/site/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/site/nonkube"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/spf13/cobra"
)

func NewCmdSite() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "site",
		Short: "A site is where skupper is deployed and components of your application are running.",
		Long:  `A site is a place where components of your application are running. Sites are linked to form application networks.`,
		Example: `skupper site create my-site
skupper site status`,
	}
	platform := common.Platform(config.GetPlatform())
	cmd.AddCommand(CmdSiteCreateFactory(platform))
	cmd.AddCommand(CmdSiteStatusFactory(platform))
	cmd.AddCommand(CmdSiteDeleteFactory(platform))
	cmd.AddCommand(CmdSiteUpdateFactory(platform))
	cmd.AddCommand(CmdSiteGenerateFactory(platform))

	return cmd
}

func CmdSiteCreateFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdSiteCreate()
	nonKubeCommand := nonkube.NewCmdSiteCreate()

	cmdSiteCreateDesc := common.SkupperCmdDescription{
		Use:   "create <name>",
		Short: "Create a new site",
		Long: `A site is a place where components of your application are running.
Sites are linked to form application networks.
There can be only one site definition per namespace.`,
		Example: "skupper site create my-site --wait configured",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSiteCreateDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandSiteCreateFlags{}

	cmd.Flags().BoolVar(&cmdFlags.EnableLinkAccess, common.FlagNameEnableLinkAccess, false, common.FlagDescEnableLinkAccess)
	cmd.Flags().StringVar(&cmdFlags.LinkAccessType, common.FlagNameLinkAccessType, "", common.FlagDescLinkAccessType)
	cmd.Flags().BoolVar(&cmdFlags.EnableHA, common.FlagNameHA, false, common.FlagDescHA)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 3*time.Minute, common.FlagDescTimeout)
	cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "ready", common.FlagDescWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	if configuredPlatform != common.PlatformKubernetes {
		cmd.Flags().MarkHidden(common.FlagNameWait)
	}

	return cmd

}

func CmdSiteUpdateFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdSiteUpdate()
	nonKubeCommand := nonkube.NewCmdSiteUpdate()

	cmdSiteUpdateDesc := common.SkupperCmdDescription{
		Use:     "update <name>",
		Short:   "Change site settings",
		Long:    `Change site settings of a given site.`,
		Example: "skupper site update my-site --enable-link-access",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSiteUpdateDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandSiteUpdateFlags{}

	cmd.Flags().BoolVar(&cmdFlags.EnableLinkAccess, common.FlagNameEnableLinkAccess, false, common.FlagDescEnableLinkAccess)
	cmd.Flags().StringVar(&cmdFlags.LinkAccessType, common.FlagNameLinkAccessType, "", common.FlagDescLinkAccessType)
	cmd.Flags().BoolVar(&cmdFlags.EnableHA, common.FlagNameHA, false, common.FlagDescHA)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 30*time.Second, common.FlagDescTimeout)
	cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "ready", common.FlagDescWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	if configuredPlatform != common.PlatformKubernetes {
		cmd.Flags().MarkHidden(common.FlagNameWait)
	}

	return cmd
}

func CmdSiteStatusFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdSiteStatus()
	nonKubeCommand := nonkube.NewCmdSiteStatus()

	cmdSiteStatusDesc := common.SkupperCmdDescription{
		Use:   "status",
		Short: "Get the site status",
		Long:  `Display the current status of a site.`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSiteStatusDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandSiteStatusFlags{}
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescConnectorStatusOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd

}

func CmdSiteDeleteFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdSiteDelete()
	nonKubeCommand := nonkube.NewCmdSiteDelete()

	cmdSiteDeleteDesc := common.SkupperCmdDescription{
		Use:   "delete",
		Short: "Delete a site",
		Long:  "Delete a site",
		Example: `skupper site delete my-site
skupper site delete my-site --wait=false
skupper site delete --all`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSiteDeleteDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandSiteDeleteFlags{}

	cmd.Flags().BoolVar(&cmdFlags.All, common.FlagNameAll, false, common.FlagDescDeleteAll)
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

func CmdSiteGenerateFactory(configuredPlatform common.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdSiteGenerate()
	nonKubeCommand := nonkube.NewCmdSiteGenerate()

	cmdSiteGenerateDesc := common.SkupperCmdDescription{
		Use:   "generate <name>",
		Short: "Generate a site resource and output it to a file or screen",
		Long: `A site is a place where components of your application are running.
Sites are linked to form application networks.
There can be only one site definition per namespace.
Generate a site resource to evaluate what will be created with the site create command`,
		Example: "skupper site generate my-site --enable-link-access",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSiteGenerateDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandSiteGenerateFlags{}

	cmd.Flags().BoolVar(&cmdFlags.EnableLinkAccess, common.FlagNameEnableLinkAccess, false, common.FlagDescEnableLinkAccess)
	cmd.Flags().StringVar(&cmdFlags.LinkAccessType, common.FlagNameLinkAccessType, "", common.FlagDescLinkAccessType)
	cmd.Flags().BoolVar(&cmdFlags.EnableHA, common.FlagNameHA, false, common.FlagDescHA)
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "yaml", common.FlagDescOutput)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd

}
