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

			siteConfig, err := cli.SiteConfigInspect(nil, nil)
			if err != nil {
				fmt.Println(err)
			}

			currentSite := siteConfig.Reference.UID

			if sites != nil && len(sites) > 0 {
				siteList := formatter.NewList()
				siteList.Item("Sites:")
				for _, site := range sites {

					if site.Name != selectedSite && selectedSite != "all" {
						continue
					}

					if site.Namespace != selectedNamespace && selectedNamespace != "all" {
						continue
					}

					location := "remote"

					if site.SiteId == currentSite {
						location = "local"
					}

					newItem := fmt.Sprintf("[%s] %s - %s ", location, site.SiteId, site.Name)

					if len(site.Links) > 0 {
						newItem = newItem + fmt.Sprintf("- linked to %s", site.Links)
					}
					newItem = newItem + fmt.Sprintln()
					detailsMap := map[string]string{"name": site.Name, "namespace": site.Namespace, "URL": site.Url}

					services := siteList.NewChildWithDetail(newItem, detailsMap)
					if len(site.Services) > 0 {
						for _, svc := range site.Services {
							svcItem := "service name " + svc.Name + fmt.Sprintln()
							detailsSvc := map[string]string{"protocol": svc.Protocol, "address": svc.Address}
							targets := services.NewChildWithDetail(svcItem, detailsSvc)

							if len(svc.Targets) > 0 {
								for _, target := range svc.Targets {
									targets.NewChild("target " + target.Name)

								}
							}

						}
					}
				}

				siteList.Print()
			} else {
				fmt.Printf("Network has no reachable sites")
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&selectedSite, "site", "s", "all", "Site identifier")
	cmd.Flags().StringVarP(&selectedNamespace, "namespace", "n", "all", "Namespace to filter")

	return cmd

}
