/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

import (
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/spf13/cobra"
)

func NewCmdSite() *cobra.Command {

	cmd := &cobra.Command{
		Use: "site",
		Long: `A site is a location where components of your application are running. 
Sites are linked together to form a network. They have different kinds 
based on platform link Kubernetes, Podman, virtual machines, and bare metal hosts.`,
		Example: `skupper site create --name my-site
skupper site get my-site`,
	}

	siteCreateCommand := NewCmdSiteCreate()
	siteGetCommand := NewCmdSiteGet()
	siteUpdateCommand := NewCmdSiteUpdate()

	cmd.AddCommand(&siteCreateCommand.CobraCmd)
	cmd.AddCommand(&siteGetCommand.CobraCmd)
	cmd.AddCommand(&siteUpdateCommand.CobraCmd)

	cmd.PersistentFlags().StringVarP(&config.Platform, "platform", "", "kubernetes", "The platform type to use [kubernetes, podman]")

	return cmd
}
