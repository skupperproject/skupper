package main

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network [command]",
		Short: "Show information about the sites and services included in the network.",
	}
	return cmd
}

var selectedSite string
var selectedNamespace string
var allNamespacesSelected bool

func NewCmdNetworkStatus(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			sites, err := cli.NetworkStatus()

			if err != nil {
				fmt.Println(err)
			}

			if sites != nil && len(*sites) > 0 {
				siteList := formatter.NewList()
				siteList.Item("Sites:")
				for _, site := range *sites {
					newItem := fmt.Sprintf("%s - %s ", site.SiteId, site.Name)
					if len(site.Links) > 0 {
						newItem = newItem + fmt.Sprintf("- linked to %s", site.Links)
					}
					newItem = newItem + fmt.Sprintln()
					detailsMap := map[string]string { "name": site.Name, "namespace": site.Namespace, "URL": site.Url}
					siteList.NewChildWithDetail(newItem, detailsMap)
				}
				siteList.Print()
			} else {
				fmt.Printf("Network has no reachable sites")
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&selectedSite, "site", "s", "", "Site identifier")
	cmd.Flags().StringVarP(&selectedNamespace, "namespace", "n", "", "Namespace to filter")
	cmd.Flags().BoolVarP(&allNamespacesSelected, "all-namespaces", "", false, "To show the sites from all the namespaces the user has permission")
	return cmd

}
