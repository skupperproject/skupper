package main

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network [command]",
		Short: "Show information about the sites and services included in the network.",
	}
	return cmd
}

var selectedSite string

func NewCmdNetworkStatus(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			var sites []*types.SiteInfo
			var errStatus error
			err := utils.Retry(time.Second, 30, func() (bool, error) {
				sites, errStatus = cli.NetworkStatus()

				if errStatus != nil {
					return false, errStatus
				}

				return true, nil
			})

			if err != nil {
				fmt.Printf("Unable to retrieve network information: %s", err)
				fmt.Println()
				return nil
			}

			siteConfig, err := cli.SiteConfigInspect(nil, nil)
			if err != nil || siteConfig == nil {
				fmt.Printf("The site configuration is not available: %s", err)
				fmt.Println()
				return nil
			}

			currentSite := siteConfig.Reference.UID

			if sites != nil && len(sites) > 0 {
				siteList := formatter.NewList()
				siteList.Item("Sites:")
				for _, site := range sites {

					if site.Name != selectedSite && selectedSite != "all" {
						continue
					}

					location := "remote"
					siteVersion := site.Version

					if len(site.MinimumVersion) > 0 {
						siteVersion = fmt.Sprintf("%s (minimum version required %s)", site.Version, site.MinimumVersion)
					}

					if site.SiteId == currentSite {
						location = "local"
					}

					newItem := fmt.Sprintf("[%s] %s - %s ", location, site.SiteId[:7], site.Name)

					newItem = newItem + fmt.Sprintln()
					detailsMap := map[string]string{"name": site.Name, "namespace": site.Namespace, "URL": site.Url, "version": siteVersion}

					if len(site.Links) > 0 {
						detailsMap["sites linked to"] = fmt.Sprint(strings.Join(site.Links, ", "))
					}

					serviceLevel := siteList.NewChildWithDetail(newItem, detailsMap)
					if len(site.Services) > 0 {
						services := serviceLevel.NewChild("Services:")
						for _, svc := range site.Services {
							svcItem := "name: " + svc.Name + fmt.Sprintln()
							detailsSvc := map[string]string{"protocol": svc.Protocol, "address": svc.Address}
							targetLevel := services.NewChildWithDetail(svcItem, detailsSvc)

							if len(svc.Targets) > 0 {
								targets := targetLevel.NewChild("Targets:")
								for _, target := range svc.Targets {
									targets.NewChild("name: " + target.Name)

								}
							}

						}
					}
				}

				siteList.Print()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&selectedSite, "site", "s", "all", "Site identifier")

	return cmd

}
