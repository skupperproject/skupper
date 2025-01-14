/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

import (
	"time"

	"github.com/skupperproject/skupper/api/types"
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

	cmd.AddCommand(CmdSiteCreateFactory(config.GetPlatform()))
	cmd.AddCommand(CmdSiteStatusFactory(config.GetPlatform()))
	cmd.AddCommand(CmdSiteDeleteFactory(config.GetPlatform()))
	cmd.AddCommand(CmdSiteUpdateFactory(config.GetPlatform()))

	return cmd
}

func CmdSiteCreateFactory(configuredPlatform types.Platform) *cobra.Command {
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
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)
	cmd.Flags().StringVar(&cmdFlags.ServiceAccount, common.FlagNameServiceAccount, "", common.FlagDescServiceAccount)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 30*time.Second, common.FlagDescTimeout)
	cmd.Flags().StringVar(&cmdFlags.BindHost, common.FlagNameBindHost, "0.0.0.0", common.FlagDescBindHost)
	cmd.Flags().StringSliceVar(&cmdFlags.SubjectAlternativeNames, common.FlagNameSubjectAlternativeNames, []string{}, common.FlagDescSubjectAlternativeNames)
	cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "ready", common.FlagDescWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	if configuredPlatform == types.PlatformKubernetes {
		cmd.Flags().MarkHidden(common.FlagNameBindHost)
		cmd.Flags().MarkHidden(common.FlagNameSubjectAlternativeNames)
	} else {
		cmd.Flags().MarkHidden(common.FlagNameServiceAccount)
		cmd.Flags().MarkHidden(common.FlagNameWait)
	}

	return cmd

}

func CmdSiteUpdateFactory(configuredPlatform types.Platform) *cobra.Command {
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
	cmd.Flags().StringVarP(&cmdFlags.Output, common.FlagNameOutput, "o", "", common.FlagDescOutput)
	cmd.Flags().StringVar(&cmdFlags.ServiceAccount, common.FlagNameServiceAccount, "", common.FlagDescServiceAccount)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 30*time.Second, common.FlagDescTimeout)
	cmd.Flags().StringVar(&cmdFlags.BindHost, common.FlagNameBindHost, "", common.FlagDescBindHost)
	cmd.Flags().StringSliceVar(&cmdFlags.SubjectAlternativeNames, common.FlagNameSubjectAlternativeNames, []string{}, common.FlagDescSubjectAlternativeNames)
	cmd.Flags().StringVar(&cmdFlags.Wait, common.FlagNameWait, "ready", common.FlagDescWait)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	if configuredPlatform == types.PlatformKubernetes {
		cmd.Flags().MarkHidden(common.FlagNameBindHost)
		cmd.Flags().MarkHidden(common.FlagNameSubjectAlternativeNames)
	} else {
		cmd.Flags().MarkHidden(common.FlagNameServiceAccount)
		cmd.Flags().MarkHidden(common.FlagNameWait)
	}

	return cmd
}

func CmdSiteStatusFactory(configuredPlatform types.Platform) *cobra.Command {
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

func CmdSiteDeleteFactory(configuredPlatform types.Platform) *cobra.Command {
	kubeCommand := kube.NewCmdSiteDelete()
	nonKubeCommand := nonkube.NewCmdSiteDelete()

	cmdSiteDeleteDesc := common.SkupperCmdDescription{
		Use:   "delete",
		Short: "Delete a site",
		Long:  `Delete a site by name`,
		Example: `skupper site delete my-site
skupper site delete --wait=false`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSiteDeleteDesc, kubeCommand, nonKubeCommand)
	cmdFlags := common.CommandSiteDeleteFlags{}

	cmd.Flags().BoolVar(&cmdFlags.All, common.FlagNameAll, false, common.FlagDescDeleteAll)
	cmd.Flags().DurationVar(&cmdFlags.Timeout, common.FlagNameTimeout, 60*time.Second, common.FlagDescTimeout)
	if configuredPlatform == types.PlatformKubernetes {
		cmd.Flags().BoolVar(&cmdFlags.Wait, common.FlagNameWait, true, common.FlagDescDeleteWait)
	}

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}
