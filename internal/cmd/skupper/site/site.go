/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/site/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/site/non_kube"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/spf13/cobra"
)

func NewCmdSite() *cobra.Command {

	return NewCmdSiteFactory(config.GetPlatform())

}

var SelectedNamespace string
var SelectedContext string
var KubeConfigPath string

func NewCmdSiteFactory(selectedPlatform types.Platform) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "site",
		Short: "A site is where skupper is deployed and components of your application are running.",
		Long:  `A site is a place where components of your application are running. Sites are linked to form application networks.`,
		Example: `skupper site create my-site
skupper site status`,
	}

	switch selectedPlatform {
	case "kubernetes":
		return AddKubeSiteCommands(cmd)
	case "podman", "docker", "systemd":
		return AddNonKubeSiteCommands(cmd)
	default:
		{
			fmt.Printf("unsupported platform: %s\n", selectedPlatform)
			return nil
		}
	}
}

func AddKubeSiteCommands(cmd *cobra.Command) *cobra.Command {

	cmd.PersistentFlags().StringVarP(&SelectedNamespace, "namespace", "n", "", "Set the namespace")
	cmd.PersistentFlags().StringVarP(&SelectedContext, "context", "c", "", "Set the kubeconfig context")
	cmd.PersistentFlags().StringVarP(&KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")

	cmd.AddCommand(&kube.NewCmdSiteCreate().CobraCmd)
	cmd.AddCommand(&kube.NewCmdSiteStatus().CobraCmd)
	cmd.AddCommand(&kube.NewCmdSiteUpdate().CobraCmd)
	cmd.AddCommand(&kube.NewCmdSiteDelete().CobraCmd)
	return cmd
}

func AddNonKubeSiteCommands(cmd *cobra.Command) *cobra.Command {

	cmd.AddCommand(&non_kube.NewCmdSiteCreate().CobraCmd)
	return cmd
}
