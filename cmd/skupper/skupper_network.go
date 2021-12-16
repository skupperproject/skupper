package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
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
			err := utils.RetryError(time.Second, 30, func() error {
				sites, errStatus = cli.NetworkStatus()

				if errStatus != nil {
					return errStatus
				}

				return nil
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
						addresses := []string{}
						svcAuth := map[string]bool{}
						for _, svc := range site.Services {
							addresses = append(addresses, svc.Name)
							svcAuth[svc.Name] = true
						}
						if vc, ok := cli.(*client.VanClient); ok && site.Namespace == cli.GetNamespace() {
							policy := client.NewPolicyValidatorAPI(vc)
							res, _ := policy.Services(addresses...)
							for addr, auth := range res {
								svcAuth[addr] = auth.Allowed
							}
						}
						for _, svc := range site.Services {
							authSuffix := ""
							if !svcAuth[svc.Name] {
								authSuffix = " - not authorized"
							}
							svcItem := "name: " + svc.Name + authSuffix + fmt.Sprintln()
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
