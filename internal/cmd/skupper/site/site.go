/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/site/kube"
	"github.com/spf13/cobra"
)

func NewCmdSite() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "site",
		Short: "A site is where skupper is deployed and components of your application are running.",
		Long:  `A site is a place where components of your application are running. Sites are linked to form application networks.`,
		Example: `skupper site create my-site
skupper site get my-site`,
	}

	siteCreateCommand := kube.NewCmdSiteCreate()
	siteGetCommand := kube.NewCmdSiteGet()
	siteUpdateCommand := kube.NewCmdSiteUpdate()
	siteDeleteCommand := kube.NewCmdSiteDelete()

	cmd.AddCommand(&siteCreateCommand.CobraCmd)
	cmd.AddCommand(&siteGetCommand.CobraCmd)
	cmd.AddCommand(&siteUpdateCommand.CobraCmd)
	cmd.AddCommand(&siteDeleteCommand.CobraCmd)

	return cmd
}
